package codesmells

import (
	"math"
	"path/filepath"
	"strings"
)

// Code health is 10 less what the code alone says will make it costly to
// change, never below 1. Each deduction was chosen for predicting later bug
// fixes beyond what size already predicts, on fifteen codebases scored as
// they stood two years earlier (tasks/code-health/DECISION.md):
//
//   - Complex code: lines in functions whose cognitive complexity passes 15.
//     The one trait that makes every edit likelier to be a bug fix.
//   - Coupling: import statements past ten. More fixes land in a file that
//     has more to know about.
//   - Size: code lines past 150. More code, more fixes, though not worse
//     code line for line.
//
// A file no language pack parsed is read by its indentation instead, when it
// is written in a programming language: size by its non-blank lines, and
// deep code -- lines three levels in -- standing in for complex code.
// Anything else gets no reading at all.
const (
	sizeFrom          = 150 // code lines
	fallbackSizeFrom  = 180 // non-blank lines, comments included: about 150 code lines
	importsFrom       = 10
	couplingWeight    = 1.5
	complexLinesScale = 25
)

// healthBreakdown is a file's health with the deductions that made it.
type healthBreakdown struct {
	health                      float64
	size, coupling, complexCode float64
	deepCode                    float64
	fallback                    bool
}

// healthOf reads m's health, or reports false when the file gets none.
func healthOf(m fileMetrics, path string) (healthBreakdown, bool) {
	var b healthBreakdown
	switch {
	case m.parsed:
		b.size = doublingsPast(float64(m.codeLines), sizeFrom)
		b.coupling = couplingWeight * doublingsPast(float64(m.imports), importsFrom)
		b.complexCode = math.Log2(1 + float64(m.complexLines)/complexLinesScale)
	case isFallbackLanguage(path):
		b.fallback = true
		b.size = doublingsPast(float64(m.nonBlank), fallbackSizeFrom)
		b.deepCode = math.Log2(1 + float64(m.deepLines)/complexLinesScale)
	default:
		return b, false
	}
	b.health = math.Max(1, 10-b.size-b.coupling-b.complexCode-b.deepCode)
	return b, true
}

// doublingsPast is how many times v doubles past from: 0 up to from.
func doublingsPast(v, from float64) float64 {
	return math.Log2(math.Max(v, from) / from)
}

// fallbackLanguages are the programming languages no language pack reads.
// Markup, data, stylesheets, SQL and shell scripts are not among them: their
// indentation says nothing about how hard they are to change.
var fallbackLanguages = map[string]bool{
	".c": true, ".cc": true, ".cpp": true, ".cxx": true, ".hpp": true, ".hh": true, ".hxx": true,
	".rs": true, ".rb": true, ".scala": true, ".sc": true, ".groovy": true, ".lua": true,
	".pl": true, ".pm": true, ".r": true, ".jl": true, ".ex": true, ".exs": true, ".erl": true,
	".hs": true, ".ml": true, ".fs": true, ".fsx": true, ".clj": true, ".cljs": true, ".elm": true,
	".nim": true, ".zig": true, ".vb": true, ".pas": true, ".cr": true, ".gd": true,
}

func isFallbackLanguage(path string) bool {
	return fallbackLanguages[strings.ToLower(filepath.Ext(path))]
}
