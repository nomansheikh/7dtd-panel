package main

import "gopkg.in/yaml.v3"

// Helpers for working with yaml.Node mappings. The bundler edits nodes
// rather than unmarshalling into maps so that key order — and therefore the
// diff of the generated output — stays stable between runs.

func mapGet(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func mapSet(n *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content[i+1] = val
			return
		}
	}
	n.Content = append(n.Content, scalar(key), val)
}

func mapDel(n *yaml.Node, key string) bool {
	if n == nil || n.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content = append(n.Content[:i:i], n.Content[i+2:]...)
			return true
		}
	}
	return false
}

// mapEnsure returns the mapping at key, creating it when absent.
func mapEnsure(n *yaml.Node, key string) *yaml.Node {
	if v := mapGet(n, key); v != nil {
		return v
	}
	v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapSet(n, key, v)
	return v
}

func scalar(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func boolNode(b bool) *yaml.Node {
	v := "false"
	if b {
		v = "true"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
}

// walk visits n and every descendant, parents before children.
func walk(n *yaml.Node, fn func(*yaml.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for _, c := range n.Content {
		walk(c, fn)
	}
}
