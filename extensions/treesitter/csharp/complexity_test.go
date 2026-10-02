package csharp

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
)

func TestCSharpCognitiveComplexity(t *testing.T) {
	stats, fns := complexity.MeasureByName(tree_sitter.NewLanguage(csharp.Language()), `using System;
using System.Linq;

namespace Shop
{
    public class Cart
    {
        public int Count { get { if (items == null) { return 0; } return items.Length; } } // +1

        public void M(int a)
        {
            if (a > 0)                                   // +1
            {
                foreach (var x in xs) { }                // +2
            }
            else if (a < 0) { }                          // +1
            else { }                                     // +1
            var y = a > 1 ? 1 : 2;                       // +1
        }
    }
}
`)
	if m := fns["Cart.M"]; m == nil || m.Cognitive != 6 || m.Params != 1 {
		t.Errorf("Cart.M: %+v", m)
	}
	if stats["modularity__imports__count"] != 2 || stats["complexity__functions"] != 2 {
		t.Errorf("stats %v, functions %v", stats, fns)
	}
}
