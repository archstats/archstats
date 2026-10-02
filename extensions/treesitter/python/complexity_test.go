package python

import (
	"strings"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

func TestPythonCognitiveComplexity(t *testing.T) {
	stats, fns := complexity.MeasureByName(tree_sitter.NewLanguage(python.Language()), `import os
from typing import List, Dict

class C:
    """A docstring is a string, not a comment."""

    def f(self, x, y=1):
        if x:            # +1
            for i in y:  # +2
                pass
            else:        # a loop's else costs nothing
                pass
        elif y:          # +1
            pass
        else:            # +1
            pass
        return x and y or x  # +2: two different operators
`)
	if f := fns["C.f"]; f == nil || f.Cognitive != 7 || f.Params != 2 {
		t.Errorf("C.f: %+v", f)
	}
	var constructs []string
	for _, i := range fns["C.f"].Increments {
		constructs = append(constructs, i.Construct)
	}
	if got := strings.Join(constructs, " "); got != "if for elif else or and" {
		t.Errorf("C.f constructs: %s", got)
	}
	if stats["modularity__imports__count"] != 2 || stats["complexity__lines__code"] != 15 {
		t.Errorf("stats %v", stats)
	}
}
