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

// An import that spells out the file's extension names the module without
// it: `./ChatPanel.vue` is the only way to import a Vue component, and ESM
// TypeScript writes `./reader.js` for `reader.ts`.
func TestAnImportSpellingOutItsExtensionResolves(t *testing.T) {
	units := []*Unit{
		{ID: "src/app#", Kind: KindModule, Files: []string{"src/app.ts"}, Refs: []Ref{
			{Module: "./reader.js", Name: "parse"},
			{Module: "./components/index.js", Name: "Button"},
			{Module: "~/components/ChatPanel.vue", Name: "ChatPanel"},
		}},
		{ID: "src/reader#parse", Kind: KindFunction, Name: "parse", Files: []string{"src/reader.ts"}},
		{ID: "src/components#Button", Kind: KindFunction, Name: "Button", Files: []string{"src/components/index.ts"}},
		{ID: "src/components/ChatPanel#ChatPanel", Kind: KindType, Name: "ChatPanel", Files: []string{"src/components/ChatPanel.vue"}},
	}
	got := map[string]bool{}
	for _, c := range Connections(units) {
		got[c.To] = true
	}
	for _, want := range []string{"src/reader#parse", "src/components#Button", "src/components/ChatPanel#ChatPanel"} {
		if !got[want] {
			t.Errorf("no edge to %s; got %v", want, got)
		}
	}
	// A dotted name is not a file path, and keeps its last segment.
	if withoutSourceExtension("acme.js") != "acme.js" {
		t.Errorf("a dotted module name was treated as a file")
	}
}
