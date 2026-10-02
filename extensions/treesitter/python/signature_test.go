package python

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

func TestPythonSignature(t *testing.T) {
	_, fns := complexity.MeasureByName(tree_sitter.NewLanguage(python.Language()), `
class OrderView(View):
    @login_required
    def post(self, request, *args, **kwargs) -> HttpResponse:
        return None
`)
	if got, want := fns["OrderView.post"].Signature, "def post(self, request, *args, **kwargs) -> HttpResponse:"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}
