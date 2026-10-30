package ecmascript

import (
	"testing"

	"github.com/archstats/archstats/core/unit"
)

// An import that spells out the file's extension names the module without
// it: `./ChatPanel.vue` is the only way to import a Vue component, and ESM
// TypeScript writes `./reader.js` for `reader.ts`. An alias names a directory
// of this project's own.
func TestAnImportSpellingOutItsExtensionResolves(t *testing.T) {
	units := []*unit.Unit{
		{ID: "src/app#", Kind: unit.KindModule, Files: []string{"src/app.ts"}, Refs: []unit.Ref{
			ImportedRef("./reader.js", "parse"),
			ImportedRef("./components/index.js", "Button"),
			ImportedRef("~/components/ChatPanel.vue", "Panel"),
		}},
		{ID: "src/reader#parse", Kind: unit.KindFunction, Name: "parse", Files: []string{"src/reader.ts"}},
		{ID: "src/components#Button", Kind: unit.KindFunction, Name: "Button", Files: []string{"src/components/index.ts"}},
		{ID: "src/components/ChatPanel#ChatPanel", Kind: unit.KindType, Name: "ChatPanel", Files: []string{"src/components/ChatPanel.vue"}},
	}
	got := map[string]bool{}
	for _, c := range unit.Connections(units) {
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
