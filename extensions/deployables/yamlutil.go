package deployables

import (
	"bytes"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAML is read as nodes rather than into structs so every fact keeps the
// line it came from, and so one malformed document in a multi-document file
// costs that document only.

// yamlDocs decodes every document in a file. A document that fails to parse
// ends the read (the decoder cannot resynchronise) but keeps what came
// before; a Helm template, full of `{{ }}`, usually yields nothing at all.
func yamlDocs(content []byte) []*yaml.Node {
	dec := yaml.NewDecoder(bytes.NewReader(content))
	var out []*yaml.Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		root := resolve(doc.Content[0])
		if root != nil {
			out = append(out, root)
		}
	}
	return out
}

func resolve(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

// get returns the value under a key of a mapping, or nil.
func get(n *yaml.Node, key string) *yaml.Node {
	n = resolve(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Value == "<<" {
			// A merge key: look inside the merged mapping too.
			if v := get(n.Content[i+1], key); v != nil {
				return v
			}
			continue
		}
		if k.Value == key {
			return resolve(n.Content[i+1])
		}
	}
	return nil
}

// at walks nested keys: at(n, "spec", "template", "spec").
func at(n *yaml.Node, keys ...string) *yaml.Node {
	for _, k := range keys {
		n = get(n, k)
		if n == nil {
			return nil
		}
	}
	return n
}

type pair struct {
	Key   string
	KeyN  *yaml.Node
	Value *yaml.Node
}

// pairs lists a mapping's entries in document order, merge keys expanded.
func pairs(n *yaml.Node) []pair {
	n = resolve(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out []pair
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Value == "<<" {
			merged := resolve(n.Content[i+1])
			if merged != nil && merged.Kind == yaml.SequenceNode {
				for _, m := range merged.Content {
					out = append(out, pairs(m)...)
				}
			} else {
				out = append(out, pairs(merged)...)
			}
			continue
		}
		out = append(out, pair{Key: k.Value, KeyN: k, Value: resolve(n.Content[i+1])})
	}
	return out
}

func items(n *yaml.Node) []*yaml.Node {
	n = resolve(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]*yaml.Node, 0, len(n.Content))
	for _, c := range n.Content {
		out = append(out, resolve(c))
	}
	return out
}

// str is a scalar's text, "" for anything else or a null.
func str(n *yaml.Node) string {
	n = resolve(n)
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

func line(n *yaml.Node) int {
	if n == nil {
		return 0
	}
	return n.Line
}

// strs reads a scalar or a sequence of scalars as a list.
func strs(n *yaml.Node) []string {
	n = resolve(n)
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		if s := str(n); s != "" {
			return []string{s}
		}
		return nil
	}
	var out []string
	for _, c := range items(n) {
		if s := str(c); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// keys of a mapping, or the scalars of a sequence: `depends_on` and
// `needs` are written both ways.
func keysOrItems(n *yaml.Node) []*yaml.Node {
	n = resolve(n)
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.MappingNode:
		var out []*yaml.Node
		for i := 0; i < len(n.Content); i += 2 {
			out = append(out, n.Content[i])
		}
		return out
	case yaml.SequenceNode:
		return items(n)
	case yaml.ScalarNode:
		return []*yaml.Node{n}
	}
	return nil
}

// A flatValue is one leaf of a YAML tree, addressed by its dotted path.
type flatValue struct {
	Key   string
	Value string
	Line  int
}

// flatten lists every scalar leaf under n with its dotted key. Sequence
// items are addressed by index: `env.0.value`.
func flatten(n *yaml.Node, prefix string) []flatValue {
	n = resolve(n)
	if n == nil {
		return nil
	}
	join := func(k string) string {
		if prefix == "" {
			return k
		}
		return prefix + "." + k
	}
	switch n.Kind {
	case yaml.MappingNode:
		var out []flatValue
		for _, p := range pairs(n) {
			out = append(out, flatten(p.Value, join(p.Key))...)
		}
		return out
	case yaml.SequenceNode:
		var out []flatValue
		for i, c := range n.Content {
			out = append(out, flatten(c, join(strconv.Itoa(i)))...)
		}
		return out
	case yaml.ScalarNode:
		if n.Tag == "!!null" {
			return []flatValue{{Key: prefix, Value: "", Line: n.Line}}
		}
		return []flatValue{{Key: prefix, Value: n.Value, Line: n.Line}}
	}
	return nil
}

// A keyValue is an environment variable or a configuration entry with the
// line it was written on.
type keyValue struct {
	Key   string
	Value string
	Line  int
}

// envEntries reads the two ways an environment is written in YAML: a
// mapping (`KEY: value`) and a list (`- KEY=value`, or `- name: K value: v`
// in Kubernetes).
func envEntries(n *yaml.Node) []keyValue {
	n = resolve(n)
	if n == nil {
		return nil
	}
	var out []keyValue
	switch n.Kind {
	case yaml.MappingNode:
		for _, p := range pairs(n) {
			out = append(out, keyValue{Key: p.Key, Value: str(p.Value), Line: line(p.KeyN)})
		}
	case yaml.SequenceNode:
		for _, it := range items(n) {
			if it.Kind == yaml.MappingNode {
				if name := str(get(it, "name")); name != "" {
					out = append(out, keyValue{Key: name, Value: str(get(it, "value")), Line: line(it)})
				}
				continue
			}
			k, v, _ := strings.Cut(str(it), "=")
			if k != "" {
				out = append(out, keyValue{Key: k, Value: v, Line: line(it)})
			}
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
