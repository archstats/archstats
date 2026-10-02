package golang

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func TestGoCognitiveComplexity(t *testing.T) {
	stats, fns := complexity.MeasureByName(tree_sitter.NewLanguage(golang.Language()), `package a

import (
	"fmt"
	"strings"
)

func f(a, b int, c string) int {
	if a > 0 { // +1
		switch b { // +2
		case 1:
		case 2:
		}
	} else if b > 0 { // +1
	} else { // +1
	}
	return a
}

func (s *Store) Get(key string) string {
	go func() {
		if key == "" || s == nil { // +2 (nested in the closure), +1 for ||
			fmt.Println(strings.ToUpper(key))
		}
	}()
	return key
}
`)
	if f := fns["f"]; f == nil || f.Cognitive != 5 || f.Params != 3 {
		t.Errorf("f: %+v", f)
	}
	if g := fns["Store.Get"]; g == nil || g.Cognitive != 3 || g.Params != 1 {
		t.Errorf("Store.Get: %+v", g)
	}
	if stats["modularity__imports__count"] != 2 || stats["complexity__functions"] != 2 {
		t.Errorf("stats %v", stats)
	}
}
