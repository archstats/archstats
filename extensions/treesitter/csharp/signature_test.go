package csharp

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
)

func TestCSharpSignature(t *testing.T) {
	_, fns := complexity.MeasureByName(tree_sitter.NewLanguage(csharp.Language()), `
public class OrdersController : Controller {
    [HttpPost("place")]
    [Authorize]
    public async Task<IActionResult> Place([FromBody] Cart cart) { return null; }
}
`)
	if got, want := fns["OrdersController.Place"].Signature, "public async Task<IActionResult> Place([FromBody] Cart cart)"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}
