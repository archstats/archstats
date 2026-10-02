package codesmells

import (
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
)

// The view lists the codebase's own functions, not a vendored bundle's.
func TestFunctionsViewLeavesOutVendoredAndGeneratedCode(t *testing.T) {
	results := &core.Results{
		FunctionsByFile: map[string][]*file.Function{
			"src/cart.ts":          {{Name: "total", Begin: 3, End: 20, Cognitive: 18, Nesting: 3, Params: 2}},
			"vendor/lib.min.js":    {{Begin: 1, End: 1, Cognitive: 4000}},
			"src/api.generated.ts": {{Name: "call", Begin: 1, End: 9, Cognitive: 2}},
		},
		ThirdPartyFiles: map[string]bool{"vendor/lib.min.js": true},
		GeneratedFiles:  map[string]bool{"src/api.generated.ts": true},
		FileToComponent: map[string]string{"src/cart.ts": "cart"},
	}
	view := functionsView(results)
	if len(view.Rows) != 1 {
		t.Fatalf("rows %d, want 1", len(view.Rows))
	}
	row := view.Rows[0].Data
	if row["file"] != "src/cart.ts" || row["name"] != "total" || row["lines"] != 18 || row["cognitive"] != 18 || row["component"] != "cart" {
		t.Errorf("row %v", row)
	}
}
