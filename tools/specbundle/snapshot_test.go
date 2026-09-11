package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestBundleSnapshot runs the whole pipeline over the real vendored spec.
//
// The unit tests cover each transform in isolation against small fixtures;
// this one guards the thing that actually ships, and will fail loudly if a
// spec refresh introduces a shape the bundler does not handle.
func TestBundleSnapshot(t *testing.T) {
	files, err := readDir("../../api/snapshot")
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	// 29 files: the root manifest plus the 28 sub-specs it references.
	if len(files) != 29 {
		t.Errorf("snapshot has %d spec files, want 29 — was it refreshed?", len(files))
	}

	docs, err := parseDocs(files)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := applyPatches(docs); err != nil {
		t.Fatalf("patches: %v", err)
	}
	root, err := bundle(docs)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	downconvert(root)

	t.Run("every path ref is inlined", func(t *testing.T) {
		paths := mapGet(root, "paths")
		if paths == nil {
			t.Fatal("bundled spec has no paths")
		}
		count := 0
		for i := 0; i+1 < len(paths.Content); i += 2 {
			key, entry := paths.Content[i].Value, paths.Content[i+1]
			if mapGet(entry, "$ref") != nil {
				t.Errorf("path %s still holds a $ref", key)
			}
			count++
		}
		// The server exposes 45 documented endpoints.
		if count != 45 {
			t.Errorf("bundled %d paths, want 45", count)
		}
	})

	t.Run("no BASEPATH placeholder survives", func(t *testing.T) {
		paths := mapGet(root, "paths")
		for i := 0; i+1 < len(paths.Content); i += 2 {
			if strings.Contains(paths.Content[i].Value, "BASEPATH") {
				t.Errorf("path key %q still contains BASEPATH", paths.Content[i].Value)
			}
		}
		// The real prefixes the placeholders stand in for must be present.
		for _, want := range []string{"/session/login", "/userstatus"} {
			if mapGet(paths, want) == nil {
				t.Errorf("expected path %s in bundled spec", want)
			}
		}
	})

	t.Run("no external refs remain", func(t *testing.T) {
		walk(root, func(n *yaml.Node) {
			if n.Kind != yaml.MappingNode {
				return
			}
			r := mapGet(n, "$ref")
			if r == nil {
				return
			}
			if !strings.HasPrefix(r.Value, "#/") {
				t.Errorf("unresolved external ref: %s", r.Value)
			}
		})
	})

	t.Run("no 3.1-only constructs remain", func(t *testing.T) {
		if v := mapGet(root, "openapi"); v == nil || v.Value != "3.0.3" {
			t.Errorf("openapi = %v, want 3.0.3", v)
		}
		walk(root, func(n *yaml.Node) {
			if n.Kind != yaml.MappingNode {
				return
			}
			if mapGet(n, "const") != nil {
				t.Error("a const survived downconversion")
			}
			if t2 := mapGet(n, "type"); t2 != nil {
				if t2.Kind == yaml.SequenceNode {
					t.Error("a type array survived downconversion")
				}
				if t2.Kind == yaml.ScalarNode && t2.Value == "null" {
					t.Error("a null type survived downconversion")
				}
			}
		})
	})

	t.Run("patched upstream bugs are actually fixed", func(t *testing.T) {
		// Lowercase oneof, which parsers ignore silently.
		walk(root, func(n *yaml.Node) {
			if n.Kind == yaml.MappingNode && mapGet(n, "oneof") != nil {
				t.Error("lowercase oneof survived patching")
			}
		})

		// WebApiTokenBodyIn.secret omitted "type:" upstream, leaving the
		// property value as the bare scalar "string".
		body := mapGet(mapGet(root, "components"), "requestBodies")
		secret := mapGet(
			mapGet(mapGet(mapGet(mapGet(
				mapGet(body, "WebApiTokens_WebApiTokenBodyIn"),
				"content"), "application/json"), "schema"), "properties"),
			"secret")
		if secret == nil {
			t.Fatal("WebApiTokenBodyIn.secret is missing")
		}
		if secret.Kind != yaml.MappingNode {
			t.Fatalf("secret is %v, want a schema mapping", secret.Kind)
		}
		if got := mapGet(secret, "type"); got == nil || got.Value != "string" {
			t.Errorf("secret.type = %v, want string", got)
		}
	})
}
