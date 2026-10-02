package typescript

import (
	"testing"

	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

func TestTypeScriptCognitiveComplexity(t *testing.T) {
	stats, fns := common.JSComplexity.MeasureByName(tree_sitter.NewLanguage(typescript.LanguageTypescript()), `import { a } from "./a"
import type { B } from "./b"
const legacy = require("legacy")

export const f = (a: number, b?: string) => {
  if (a) {                                     // +1
    items.forEach((x) => { if (x) { a++ } })   // the callback nests: +3
  } else if (b) {                              // +1
  } else {                                     // +1
  }
  return a ? 1 : 2                             // +1
}

class Cart {
  total(lines: Line[]): number {
    return lines.length > 0 && lines[0].price != null ? 1 : 0 // +1 ternary, +1 &&
  }
}
`)
	if f := fns["f"]; f == nil || f.Cognitive != 7 || f.Params != 2 {
		t.Errorf("f: %+v", f)
	}
	if c := fns["Cart.total"]; c == nil || c.Cognitive != 2 || c.Params != 1 {
		t.Errorf("Cart.total: %+v", c)
	}
	if stats["modularity__imports__count"] != 3 || stats["complexity__functions"] != 2 {
		t.Errorf("stats %v", stats)
	}
}
