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

// A pack may say what a module is as a path, and under the project's own
// alias; resolution uses those, and the edge keeps what the source wrote.
func TestRefPathAndUnaliasedResolve(t *testing.T) {
	units := []*Unit{
		{ID: "src/app#", Kind: KindModule, Files: []string{"src/app.x"}, Refs: []Ref{
			{Module: "./reader.x", Path: "./reader", Name: "parse"},
			{Module: "$/components/Panel.x", Path: "$/components/Panel", Unaliased: "components/Panel", Name: "Panel"},
		}},
		{ID: "src/reader#parse", Kind: KindFunction, Name: "parse", Files: []string{"src/reader.x"}},
		{ID: "src/components/Panel#Panel", Kind: KindType, Name: "Panel", Files: []string{"src/components/Panel.x"}},
	}
	via := map[string]string{}
	for _, c := range Connections(units) {
		via[c.To] = c.Via
	}
	if via["src/reader#parse"] != "./reader.x" || via["src/components/Panel#Panel"] != "$/components/Panel.x" {
		t.Errorf("got %v", via)
	}
}

// A qualified id splits before its name, whatever separator the language
// writes between them.
func TestSplitID(t *testing.T) {
	for _, c := range []struct{ id, name, module string }{
		{`Acme\Order\Order`, "Order", `Acme\Order`},
		{"com.acme.Order", "Order", "com.acme"},
		{"lib/router.go#Engine.ServeHTTP", "ServeHTTP", "lib/router.go"},
		{"src/app#", "app", "src/app"},
		{"fooBar", "Bar", ""},
	} {
		if m, _ := splitID(&Unit{ID: c.id, Name: c.name}); m != c.module {
			t.Errorf("%s: module %q, want %q", c.id, m, c.module)
		}
	}
}
