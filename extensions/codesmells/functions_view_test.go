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

// Only a complex function's steps are kept: a simple one has nothing to explain.
func TestComplexityIncrementsViewKeepsComplexFunctions(t *testing.T) {
	steps := []file.Increment{{Line: 4, Points: 1, Construct: "if"}, {Line: 5, Points: 15, Construct: "for", Nesting: 14}}
	results := &core.Results{
		FunctionsByFile: map[string][]*file.Function{
			"src/cart.ts": {
				{Name: "total", Begin: 3, End: 20, Cognitive: 16, Increments: steps},
				{Name: "small", Begin: 22, End: 25, Cognitive: 1, Increments: steps[:1]},
			},
			"vendor/lib.min.js": {{Begin: 1, End: 1, Cognitive: 99, Increments: steps}},
		},
		ThirdPartyFiles: map[string]bool{"vendor/lib.min.js": true},
	}
	view := complexityIncrementsView(results)
	if len(view.Rows) != 2 {
		t.Fatalf("rows %d, want 2", len(view.Rows))
	}
	if r := view.Rows[1].Data; r["function"] != "total" || r["line"] != 5 || r["points"] != 15 || r["construct"] != "for" || r["function_begin"] != 3 {
		t.Errorf("row %v", r)
	}
}
