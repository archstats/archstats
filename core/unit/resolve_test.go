package unit

import "testing"

// A module unit has no name and can never be a target, but its relative
// imports still resolve against where it lives.
func TestModuleUnitResolvesRelativeImports(t *testing.T) {
	units := []*Unit{
		{ID: "src/components/index#", Kind: KindModule, Name: "index", Files: []string{"src/components/index.ts"},
			Refs: []Ref{{Module: "./Button", Name: "Button"}}},
		{ID: "src/components/Button#Button", Kind: KindFunction, Name: "Button", Files: []string{"src/components/Button.tsx"}},
	}
	conns := Connections(units)
	if len(conns) != 1 || conns[0].From != "src/components/index#" || conns[0].To != "src/components/Button#Button" {
		t.Fatalf("got %+v", conns)
	}
}
