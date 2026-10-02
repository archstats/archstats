package golang

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	golang "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func TestGoSignature(t *testing.T) {
	_, fns := complexity.MeasureByName(tree_sitter.NewLanguage(golang.Language()), `package a

// Place places an order.
func (s *Service) Place(ctx context.Context, cart Cart) (*Order, error) {
	return nil, nil
}
`)
	if got, want := fns["Service.Place"].Signature, "func (s *Service) Place(ctx context.Context, cart Cart) (*Order, error)"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}
