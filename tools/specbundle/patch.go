package main

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// A patch fixes a specific, located bug in the upstream spec.
//
// Every patch must match. An unmatched patch is a hard error rather than a
// warning: if upstream fixes, moves or renames one of these, the build
// should stop and make a human look, instead of silently carrying a fix
// that no longer does anything.
type patch struct {
	file string
	why  string
	// apply reports whether it found and changed its target.
	apply func(doc *yaml.Node) bool
}

var patches = []patch{
	{
		file: "SandboxSettings.openapi.yaml",
		why:  "schema uses lowercase 'oneof', which every parser silently ignores",
		apply: func(doc *yaml.Node) bool {
			changed := false
			walk(doc, func(n *yaml.Node) {
				if n.Kind != yaml.MappingNode {
					return
				}
				if v := mapGet(n, "oneof"); v != nil {
					mapDel(n, "oneof")
					mapSet(n, "oneOf", v)
					changed = true
				}
			})
			return changed
		},
	},
	{
		file: "SessionHandler.openapi.yaml",
		why:  "media type written as 'plain/text' instead of 'text/plain'",
		apply: func(doc *yaml.Node) bool {
			changed := false
			walk(doc, func(n *yaml.Node) {
				if n.Kind != yaml.MappingNode {
					return
				}
				if v := mapGet(n, "plain/text"); v != nil {
					mapDel(n, "plain/text")
					mapSet(n, "text/plain", v)
					changed = true
				}
			})
			return changed
		},
	},
	{
		file: "WebApiTokens.openapi.yaml",
		why:  "WebApiTokenBodyIn.secret omits 'type:', so it parses as the bare scalar \"string\"",
		apply: func(doc *yaml.Node) bool {
			changed := false
			walk(doc, func(n *yaml.Node) {
				if n.Kind != yaml.MappingNode {
					return
				}
				props := mapGet(n, "properties")
				if props == nil || props.Kind != yaml.MappingNode {
					return
				}
				for i := 0; i+1 < len(props.Content); i += 2 {
					v := props.Content[i+1]
					// A property whose whole value is a type name is the bug.
					if v.Kind != yaml.ScalarNode {
						continue
					}
					switch v.Value {
					case "string", "integer", "number", "boolean", "object", "array":
						props.Content[i+1] = &yaml.Node{
							Kind:    yaml.MappingNode,
							Tag:     "!!map",
							Content: []*yaml.Node{scalar("type"), scalar(v.Value)},
						}
						changed = true
					}
				}
			})
			return changed
		},
	},
	{
		file: rootName,
		why:  "TypeVector3i declares integer x/y/z, but a live server sends floats such as -272.96875",
		apply: func(doc *yaml.Node) bool {
			// Observed against a real server with a player online: decoding a
			// player into the generated type fails outright with
			// "cannot unmarshal number -272.96875 into ... of type int".
			// The name says "3i" but the data is not integral.
			schemas := mapGet(mapGet(doc, "components"), "schemas")
			props := mapGet(mapGet(schemas, "TypeVector3i"), "properties")
			if props == nil {
				return false
			}
			changed := false
			for _, axis := range []string{"x", "y", "z"} {
				field := mapGet(props, axis)
				if field == nil {
					continue
				}
				if t := mapGet(field, "type"); t != nil && t.Value == "integer" {
					t.Value = "number"
					changed = true
				}
			}
			return changed
		},
	},
	{
		file: "Player.openapi.yaml",
		why:  "PlayerElement.level is typed null, but a live server sends an integer",
		apply: func(doc *yaml.Node) bool {
			// Also observed only with a player online. totalPlayTimeSeconds and
			// lastOnline really are always null on this build, so they are left
			// alone; level is not.
			schemas := mapGet(mapGet(doc, "components"), "schemas")
			props := mapGet(mapGet(schemas, "PlayerElement"), "properties")
			level := mapGet(props, "level")
			if level == nil {
				return false
			}
			t := mapGet(level, "type")
			if t == nil || t.Value != "null" {
				return false
			}
			t.Value = "integer"
			return true
		},
	},
	{
		file: rootName,
		why:  "TypeUserIdString pattern escapes '[', so it can never match any input",
		apply: func(doc *yaml.Node) bool {
			const broken = "^\\[a-zA-Z]+_[\\w]+$"
			const fixed = "^[a-zA-Z]+_[\\w]+$"
			changed := false
			walk(doc, func(n *yaml.Node) {
				if n.Kind != yaml.MappingNode {
					return
				}
				if p := mapGet(n, "pattern"); p != nil && p.Value == broken {
					p.Value = fixed
					changed = true
				}
			})
			return changed
		},
	},
}

func applyPatches(docs map[string]*yaml.Node) ([]string, error) {
	var applied []string
	var missing []string

	for _, p := range patches {
		doc := docs[p.file]
		if doc == nil {
			missing = append(missing, fmt.Sprintf("%s (file absent): %s", p.file, p.why))
			continue
		}
		if p.apply(doc) {
			applied = append(applied, fmt.Sprintf("%s — %s", p.file, p.why))
			continue
		}
		missing = append(missing, fmt.Sprintf("%s: %s", p.file, p.why))
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return applied, fmt.Errorf("these patches no longer apply, so upstream has changed "+
			"and they need review:\n  - %s", strings.Join(missing, "\n  - "))
	}
	return applied, nil
}
