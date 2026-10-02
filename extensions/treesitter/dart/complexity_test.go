package dart

import (
	"testing"

	dart "github.com/UserNobody14/tree-sitter-dart/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestDartCognitiveComplexity(t *testing.T) {
	stats, fns := complexity.MeasureByName(tree_sitter.NewLanguage(dart.Language()), `import 'package:a/b.dart';

class Cart {
  int total(int a, String? b) {
    if (a > 0 && b != null) { for (var i = 0; i < a; i++) { } } else if (a < 0) { } else { } // 1+1+2+1+1
    while (a > 0) { }                            // +1
    switch (a) { case 1: break; default: break; } // +1
    try { } catch (e) { }                        // +1
    items.forEach((x) { if (x) { } });           // +2: the closure nests
    return a > 1 ? 1 : 2;                        // +1
  }
}

int top() => 1;
`)
	if f := fns["Cart.total"]; f == nil || f.Cognitive != 12 || f.Params != 2 || f.Begin != 4 {
		t.Errorf("Cart.total: %+v (all: %v)", f, fns)
	}
	if f := fns["top"]; f == nil || f.Cognitive != 0 || f.Params != 0 {
		t.Errorf("top: %+v", f)
	}
	if stats["modularity__imports__count"] != 1 || stats["complexity__functions"] != 2 {
		t.Errorf("stats %v", stats)
	}
}
