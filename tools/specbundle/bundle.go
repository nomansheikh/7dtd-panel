package main

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// bundle flattens every sub-spec into the root document.
//
// Sub-spec components are namespaced by their source file ("Command_" +
// "CommandElement") because several files declare same-named schemas. The
// root's own components keep their bare names, since they are the shared
// Type* definitions that everything else points at.
func bundle(docs map[string]*yaml.Node) (*yaml.Node, error) {
	root := docs[rootName]
	rootComponents := mapEnsure(root, "components")

	subs := make([]string, 0, len(docs))
	for name := range docs {
		if name != rootName {
			subs = append(subs, name)
		}
	}
	sort.Strings(subs)

	for _, file := range subs {
		ns := strings.TrimSuffix(file, ".openapi.yaml")
		doc := docs[file]

		rewriteRefs(doc, ns)

		comps := mapGet(doc, "components")
		if comps == nil {
			continue
		}
		for _, section := range componentSections {
			src := mapGet(comps, section)
			if src == nil || src.Kind != yaml.MappingNode {
				continue
			}
			dst := mapEnsure(rootComponents, section)
			for i := 0; i+1 < len(src.Content); i += 2 {
				key := ns + "_" + src.Content[i].Value
				if mapGet(dst, key) != nil {
					return nil, fmt.Errorf("component name collision at %s/%s", section, key)
				}
				mapSet(dst, key, src.Content[i+1])
			}
		}
	}

	paths := mapGet(root, "paths")
	if paths == nil {
		return nil, fmt.Errorf("root spec has no paths")
	}
	resolved := 0
	for i := 0; i+1 < len(paths.Content); i += 2 {
		pathKey := paths.Content[i].Value
		entry := paths.Content[i+1]

		ref := mapGet(entry, "$ref")
		if ref == nil {
			continue
		}
		file, ptr, err := splitRef(ref.Value)
		if err != nil {
			return nil, fmt.Errorf("path %s: %w", pathKey, err)
		}
		doc := docs[file]
		if doc == nil {
			return nil, fmt.Errorf("path %s references unknown file %s", pathKey, file)
		}
		// The three handler sub-specs key their paths on a literal
		// "/BASEPATH/" placeholder; the pointer encodes that same literal, so
		// it resolves directly and the real prefix comes from the root key.
		target, err := resolvePointer(doc, ptr)
		if err != nil {
			return nil, fmt.Errorf("path %s: %w", pathKey, err)
		}
		paths.Content[i+1] = target
		resolved++
	}
	fmt.Printf("bundled: %d sub-specs merged, %d path refs inlined\n", len(subs), resolved)
	return root, nil
}

func splitRef(ref string) (file, pointer string, err error) {
	i := strings.Index(ref, "#")
	if i < 0 {
		return "", "", fmt.Errorf("ref %q has no fragment", ref)
	}
	file = strings.TrimPrefix(ref[:i], "./")
	if file == "" {
		return "", "", fmt.Errorf("ref %q is local, expected a file", ref)
	}
	return file, ref[i:], nil
}

func resolvePointer(doc *yaml.Node, pointer string) (*yaml.Node, error) {
	p := strings.TrimPrefix(pointer, "#")
	if p == "" || p == "/" {
		return doc, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("pointer %q must start with /", pointer)
	}
	cur := doc
	for _, raw := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		tok := strings.ReplaceAll(raw, "~1", "/")
		tok = strings.ReplaceAll(tok, "~0", "~")
		next := mapGet(cur, tok)
		if next == nil {
			return nil, fmt.Errorf("pointer %q: no member %q", pointer, tok)
		}
		cur = next
	}
	return cur, nil
}

// rewriteRefs repoints every $ref inside a sub-spec at the bundled root.
func rewriteRefs(doc *yaml.Node, ns string) {
	walk(doc, func(n *yaml.Node) {
		if n.Kind != yaml.MappingNode {
			return
		}
		r := mapGet(n, "$ref")
		if r == nil || r.Kind != yaml.ScalarNode {
			return
		}
		r.Value = rewriteRef(r.Value, ns)
	})
}

func rewriteRef(ref, ns string) string {
	if strings.HasPrefix(ref, "#/") {
		return namespaceRef(ref, ns)
	}
	i := strings.Index(ref, "#")
	if i < 0 {
		return ref
	}
	file := strings.TrimPrefix(ref[:i], "./")
	frag := ref[i:]
	if file == rootName {
		// Root components keep their bare names.
		return frag
	}
	return namespaceRef(frag, strings.TrimSuffix(file, ".openapi.yaml"))
}

func namespaceRef(frag, ns string) string {
	parts := strings.Split(strings.TrimPrefix(frag, "#/"), "/")
	if len(parts) == 3 && parts[0] == "components" {
		return "#/components/" + parts[1] + "/" + ns + "_" + parts[2]
	}
	return frag
}
