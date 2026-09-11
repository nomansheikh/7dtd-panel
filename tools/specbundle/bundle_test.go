package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRewriteRef(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		ns   string
		want string
	}{
		{
			name: "local component gets namespaced",
			ref:  "#/components/schemas/CommandElement",
			ns:   "Command",
			want: "#/components/schemas/Command_CommandElement",
		},
		{
			name: "root component keeps its bare name",
			ref:  "./openapi.yaml#/components/schemas/TypeVector3i",
			ns:   "Player",
			want: "#/components/schemas/TypeVector3i",
		},
		{
			name: "root response keeps its bare name",
			ref:  "./openapi.yaml#/components/responses/HttpEmptyEnvelopedResponse",
			ns:   "Markers",
			want: "#/components/responses/HttpEmptyEnvelopedResponse",
		},
		{
			name: "cross-file ref uses the other file's namespace",
			ref:  "./Command.openapi.yaml#/components/schemas/CommandsList",
			ns:   "Log",
			want: "#/components/schemas/Command_CommandsList",
		},
		{
			name: "requestBodies section is namespaced too",
			ref:  "#/components/requestBodies/MarkersBodyIn",
			ns:   "Markers",
			want: "#/components/requestBodies/Markers_MarkersBodyIn",
		},
		{
			name: "non-component pointer is left alone",
			ref:  "#/paths/~1api~1command",
			ns:   "Command",
			want: "#/paths/~1api~1command",
		},
		{
			name: "ref without a fragment is left alone",
			ref:  "./openapi.yaml",
			ns:   "Command",
			want: "./openapi.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteRef(tt.ref, tt.ns); got != tt.want {
				t.Errorf("rewriteRef(%q, %q) = %q, want %q", tt.ref, tt.ns, got, tt.want)
			}
		})
	}
}

func TestResolvePointer(t *testing.T) {
	const src = `
paths:
  /api/command:
    get: {summary: real path}
  /BASEPATH/login:
    post: {summary: placeholder path}
  /BASEPATH/:
    get: {summary: bare placeholder}
  /api/OpenAPI/openapi.yaml:
    get: {summary: dotted path}
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("fixture does not parse: %v", err)
	}
	root := doc.Content[0]

	tests := []struct {
		name        string
		pointer     string
		wantSummary string
		wantErr     bool
	}{
		{
			name:        "escaped slashes decode",
			pointer:     "#/paths/~1api~1command",
			wantSummary: "real path",
		},
		{
			// The three handler sub-specs key their paths on a literal
			// BASEPATH placeholder, so the pointer resolves without
			// substitution and the real prefix comes from the root key.
			name:        "BASEPATH placeholder resolves literally",
			pointer:     "#/paths/~1BASEPATH~1login",
			wantSummary: "placeholder path",
		},
		{
			name:        "trailing slash placeholder resolves",
			pointer:     "#/paths/~1BASEPATH~1",
			wantSummary: "bare placeholder",
		},
		{
			name:        "dots in a path segment are not special",
			pointer:     "#/paths/~1api~1OpenAPI~1openapi.yaml",
			wantSummary: "dotted path",
		},
		{
			name:    "missing member is an error",
			pointer: "#/paths/~1api~1nope",
			wantErr: true,
		},
		{
			name:    "pointer must be rooted",
			pointer: "#paths",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePointer(root, tt.pointer)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolvePointer(%q) succeeded, want error", tt.pointer)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePointer(%q): %v", tt.pointer, err)
			}
			method := got.Content[1]
			if summary := mapGet(method, "summary"); summary == nil || summary.Value != tt.wantSummary {
				t.Errorf("resolved to %v, want summary %q", summary, tt.wantSummary)
			}
		})
	}
}

func TestApplyPatchesFailsWhenAPatchNoLongerMatches(t *testing.T) {
	// A root document with none of the known bugs present, standing in for a
	// future upstream release that fixed them. The bundler must refuse rather
	// than silently carry dead patches.
	docs, err := parseDocs(map[string][]byte{
		rootName: []byte("openapi: 3.1.0\npaths: {}\n"),
	})
	if err != nil {
		t.Fatalf("parseDocs: %v", err)
	}

	if _, err = applyPatches(docs); err == nil {
		t.Fatal("applyPatches succeeded on a spec with no bugs to patch, want error")
	}
	if !strings.Contains(err.Error(), "no longer apply") {
		t.Errorf("error should explain that upstream changed, got: %v", err)
	}
}

func TestDownconvert(t *testing.T) {
	const src = `
openapi: 3.1.0
components:
  schemas:
    NullableString:
      type: [string, 'null']
    AlwaysNull:
      type: 'null'
    Constant:
      type: string
      const: Full
    WithExamples:
      type: integer
      examples: [171, 42]
    NullableRef:
      oneOf:
        - $ref: '#/components/schemas/Constant'
        - type: 'null'
    RefWithSiblings:
      $ref: '#/components/schemas/Constant'
      description: dropped, because 3.0 ignores it
paths:
  /thing:
    get:
      responses:
        200: {description: ok}
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("fixture does not parse: %v", err)
	}
	root := doc.Content[0]

	stats := downconvert(root)

	if v := mapGet(root, "openapi"); v == nil || v.Value != "3.0.3" {
		t.Errorf("openapi = %v, want 3.0.3", v)
	}

	schemas := mapGet(mapGet(root, "components"), "schemas")

	t.Run("type array becomes type plus nullable", func(t *testing.T) {
		n := mapGet(schemas, "NullableString")
		if got := mapGet(n, "type"); got == nil || got.Value != "string" {
			t.Errorf("type = %v, want string", got)
		}
		if got := mapGet(n, "nullable"); got == nil || got.Value != "true" {
			t.Errorf("nullable = %v, want true", got)
		}
	})

	t.Run("null-only type drops type and marks nullable", func(t *testing.T) {
		n := mapGet(schemas, "AlwaysNull")
		if got := mapGet(n, "type"); got != nil {
			t.Errorf("type = %v, want absent", got)
		}
		if got := mapGet(n, "nullable"); got == nil || got.Value != "true" {
			t.Errorf("nullable = %v, want true", got)
		}
	})

	t.Run("const becomes a single-value enum", func(t *testing.T) {
		n := mapGet(schemas, "Constant")
		if mapGet(n, "const") != nil {
			t.Error("const should be gone")
		}
		e := mapGet(n, "enum")
		if e == nil || len(e.Content) != 1 || e.Content[0].Value != "Full" {
			t.Errorf("enum = %v, want [Full]", e)
		}
	})

	t.Run("examples array collapses to a single example", func(t *testing.T) {
		n := mapGet(schemas, "WithExamples")
		if mapGet(n, "examples") != nil {
			t.Error("examples should be gone")
		}
		if got := mapGet(n, "example"); got == nil || got.Value != "171" {
			t.Errorf("example = %v, want 171", got)
		}
	})

	t.Run("nullable oneOf becomes allOf so nullable survives", func(t *testing.T) {
		// A bare $ref would lose the sibling nullable flag in 3.0, so the
		// single remaining member has to be wrapped.
		n := mapGet(schemas, "NullableRef")
		if mapGet(n, "oneOf") != nil {
			t.Error("oneOf should be gone")
		}
		all := mapGet(n, "allOf")
		if all == nil || len(all.Content) != 1 {
			t.Fatalf("allOf = %v, want one member", all)
		}
		if got := mapGet(n, "nullable"); got == nil || got.Value != "true" {
			t.Errorf("nullable = %v, want true", got)
		}
	})

	t.Run("ref siblings are dropped", func(t *testing.T) {
		n := mapGet(schemas, "RefWithSiblings")
		if mapGet(n, "description") != nil {
			t.Error("description alongside $ref should be dropped")
		}
		if mapGet(n, "$ref") == nil {
			t.Error("$ref itself must survive")
		}
	})

	t.Run("status codes become strings", func(t *testing.T) {
		responses := mapGet(mapGet(mapGet(mapGet(root, "paths"), "/thing"), "get"), "responses")
		key := responses.Content[0]
		if key.Tag != "!!str" {
			t.Errorf("status key tag = %q, want !!str", key.Tag)
		}
	})

	// nullTypes counts 1, not 2: the oneOf handler strips NullableRef's null
	// member before the walk descends into it, so only AlwaysNull is counted.
	want := dcStats{typeArrays: 1, nullTypes: 1, consts: 1, examples: 1, statusKeys: 1, refSiblings: 1}
	if stats != want {
		t.Errorf("stats = %+v, want %+v", stats, want)
	}
}
