package typescript

import (
	"testing"

	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

func TestTypeScriptSignature(t *testing.T) {
	_, fns := common.JSComplexity.MeasureByName(tree_sitter.NewLanguage(typescript.LanguageTypescript()), `
export const total = (items: Item[]): number => items.length
export function place(cart: Cart): Order { return null }
class Orders {
  @Get(":id")
  find(id: string): Order { return null }
}
`)
	for name, want := range map[string]string{
		"total":       "total = (items: Item[]): number =>",
		"place":       "function place(cart: Cart): Order",
		"Orders.find": "find(id: string): Order",
	} {
		if fns[name] == nil {
			t.Errorf("%s: not measured", name)
			continue
		}
		if got := fns[name].Signature; got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}
