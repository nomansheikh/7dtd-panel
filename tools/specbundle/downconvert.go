package main

import "gopkg.in/yaml.v3"

// dcStats counts what the downconversion touched, so a spec refresh that
// changes the shape of the upstream document is visible in the build log.
type dcStats struct {
	typeArrays  int
	nullTypes   int
	consts      int
	examples    int
	statusKeys  int
	refSiblings int
}

// downconvert rewrites an OpenAPI 3.1 document in place as 3.0.3.
//
// The upstream spec is 3.1 and leans on features 3.0 has no spelling for.
// Go generator support for 3.1 is uneven, so the bundler meets the tooling
// where it is rather than hoping:
//
//	type: [string, 'null']  ->  type: string, nullable: true
//	type: 'null'            ->  nullable: true, no type (becomes interface{})
//	const: x                ->  enum: [x]
//	examples: [a, b]        ->  example: a
//	oneOf: [X, 'null']      ->  allOf: [X], nullable: true
//
// The oneOf case needs allOf rather than a bare $ref because 3.0 ignores a
// $ref's siblings, which would silently discard the nullable flag.
func downconvert(root *yaml.Node) dcStats {
	var s dcStats

	if v := mapGet(root, "openapi"); v != nil {
		v.Value = "3.0.3"
	}

	walk(root, func(n *yaml.Node) {
		if n.Kind != yaml.MappingNode {
			return
		}

		// A sequence-valued "examples" is the 3.1 schema keyword. The
		// same-named map under components or a media type is a different
		// thing and must be left alone.
		if ex := mapGet(n, "examples"); ex != nil && ex.Kind == yaml.SequenceNode {
			mapDel(n, "examples")
			if len(ex.Content) > 0 {
				mapSet(n, "example", ex.Content[0])
				s.examples++
			}
		}

		if c := mapGet(n, "const"); c != nil {
			mapDel(n, "const")
			mapSet(n, "enum", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{c}})
			s.consts++
		}

		if t := mapGet(n, "type"); t != nil {
			switch t.Kind {
			case yaml.SequenceNode:
				keep := make([]*yaml.Node, 0, len(t.Content))
				hasNull := false
				for _, m := range t.Content {
					if m.Value == "null" {
						hasNull = true
						continue
					}
					keep = append(keep, m)
				}
				if hasNull {
					mapSet(n, "nullable", boolNode(true))
				}
				// 3.0 cannot express a union of concrete types. Every such
				// case upstream is exactly one type plus null, so taking the
				// first is lossless here; a future spec with a real union
				// would show up as a jump in this counter.
				if len(keep) == 0 {
					mapDel(n, "type")
				} else {
					mapSet(n, "type", keep[0])
				}
				s.typeArrays++
			case yaml.ScalarNode:
				if t.Value == "null" {
					mapDel(n, "type")
					mapSet(n, "nullable", boolNode(true))
					s.nullTypes++
				}
			}
		}

		for _, kw := range []string{"oneOf", "anyOf"} {
			v := mapGet(n, kw)
			if v == nil || v.Kind != yaml.SequenceNode {
				continue
			}
			keep := make([]*yaml.Node, 0, len(v.Content))
			hadNull := false
			for _, m := range v.Content {
				if isNullSchema(m) {
					hadNull = true
					continue
				}
				keep = append(keep, m)
			}
			if !hadNull {
				continue
			}
			mapSet(n, "nullable", boolNode(true))
			if len(keep) == 1 {
				mapDel(n, kw)
				mapSet(n, "allOf", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: keep})
			} else {
				v.Content = keep
			}
		}

		// 3.0 ignores anything alongside a $ref. Dropping the siblings keeps
		// the document honest about what a reader actually sees.
		if r := mapGet(n, "$ref"); r != nil && len(n.Content) > 2 {
			n.Content = []*yaml.Node{scalar("$ref"), r}
			s.refSiblings++
		}
	})

	// Status codes parse as YAML integers but OpenAPI requires strings.
	walk(root, func(n *yaml.Node) {
		if n.Kind != yaml.MappingNode {
			return
		}
		resp := mapGet(n, "responses")
		if resp == nil || resp.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(resp.Content); i += 2 {
			if k := resp.Content[i]; k.Tag == "!!int" {
				k.Tag = "!!str"
				k.Style = yaml.DoubleQuotedStyle
				s.statusKeys++
			}
		}
	})

	return s
}

func isNullSchema(n *yaml.Node) bool {
	if n.Kind != yaml.MappingNode {
		return false
	}
	t := mapGet(n, "type")
	return t != nil && t.Kind == yaml.ScalarNode && t.Value == "null"
}
