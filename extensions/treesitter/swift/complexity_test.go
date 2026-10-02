package swift

import (
	"testing"

	swift "github.com/archstats/archstats/extensions/treesitter/swift/grammar"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestSwiftCognitiveComplexity(t *testing.T) {
	stats, fns := complexity.MeasureByName(tree_sitter.NewLanguage(swift.Language()), `import Foundation

class Cart {
    func total(a: Int, b: String?) -> Int {
        if a > 0 && b != nil { for i in 0..<a { } } else if a < 0 { } else { } // 1+1+2+1+1
        while a > 0 { }                         // +1
        guard let z = b else { return 0 }       // +1
        switch a { case 1: break
        default: break }                        // +1
        do { try x() } catch { }                // +1
        items.forEach { x in if x { } }         // +2: the closure nests
        return a > 1 ? 1 : 2                    // +1
    }

    init(x: Int) { }
}
`)
	if f := fns["Cart.total"]; f == nil || f.Cognitive != 13 || f.Params != 2 {
		t.Errorf("Cart.total: %+v (all: %v)", f, fns)
	}
	if f := fns["Cart.init"]; f == nil || f.Params != 1 {
		t.Errorf("Cart.init: %+v", f)
	}
	if stats["modularity__imports__count"] != 1 || stats["complexity__functions"] != 2 {
		t.Errorf("stats %v", stats)
	}
}
