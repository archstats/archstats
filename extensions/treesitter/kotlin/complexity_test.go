package kotlin

import (
	"testing"

	kotlin "github.com/fwcd/tree-sitter-kotlin/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestKotlinCognitiveComplexity(t *testing.T) {
	stats, fns := complexity.MeasureByName(tree_sitter.NewLanguage(kotlin.Language()), `import a.b.C
import d.e.F

class Cart(val x: Int) {
    fun total(a: Int, b: String?): Int {
        if (a > 0 && b != null) { for (i in 0..a) { } } else if (a < 0) { } else { } // 1+1+2+1+1
        while (a > 0) { }                      // +1
        val y = when (a) { 1 -> 2 else -> 3 }  // +1
        try { } catch (e: Exception) { }       // +1
        list.forEach { if (it) { } }           // +2: the lambda nests
        return if (a > 1) 1 else 2             // +1, +1
    }
}

fun top() = 1
`)
	if f := fns["Cart.total"]; f == nil || f.Cognitive != 13 || f.Params != 2 {
		t.Errorf("Cart.total: %+v (all: %v)", f, fns)
	}
	if f := fns["top"]; f == nil || f.Cognitive != 0 {
		t.Errorf("top: %+v", f)
	}
	if stats["modularity__imports__count"] != 2 || stats["complexity__functions"] != 2 {
		t.Errorf("stats %v", stats)
	}
}
