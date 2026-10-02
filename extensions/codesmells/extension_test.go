package codesmells

import (
	"github.com/archstats/archstats/core/stats"
	"math"
	"testing"
)

func healthFor(t *testing.T, m fileMetrics, path string) healthBreakdown {
	t.Helper()
	b, ok := healthOf(m, path)
	if !ok {
		t.Fatalf("%s: no health reading", path)
	}
	return b
}

// A small, simple file with few imports loses nothing.
func TestHealthOfASmallSimpleFile(t *testing.T) {
	b := healthFor(t, fileMetrics{parsed: true, codeLines: 120, imports: 6}, "a.go")
	if b.health != 10 {
		t.Errorf("health %v, want 10", b.health)
	}
}

// Each deduction is a point (coupling a point and a half) per doubling past
// where it starts, and nothing caps one: a 9,600-line file loses six.
func TestHealthDeductionsDoublePerPoint(t *testing.T) {
	cases := []struct {
		m                           fileMetrics
		size, coupling, complexCode float64
	}{
		{fileMetrics{parsed: true, codeLines: 300}, 1, 0, 0},
		{fileMetrics{parsed: true, codeLines: 1200}, 3, 0, 0},
		{fileMetrics{parsed: true, codeLines: 9600}, 6, 0, 0},
		{fileMetrics{parsed: true, codeLines: 100, imports: 40}, 0, 3, 0},
		{fileMetrics{parsed: true, codeLines: 100, complexLines: 75}, 0, 0, 2},
	}
	for _, c := range cases {
		b := healthFor(t, c.m, "a.java")
		if math.Abs(b.size-c.size) > 1e-9 || math.Abs(b.coupling-c.coupling) > 1e-9 || math.Abs(b.complexCode-c.complexCode) > 1e-9 {
			t.Errorf("%+v: deductions %v %v %v, want %v %v %v", c.m, b.size, b.coupling, b.complexCode, c.size, c.coupling, c.complexCode)
		}
	}
}

// The worked example of tasks/code-health/DECISION.md: 600 code lines, 20
// imports and one 75-line complex function make 4.5.
func TestHealthWorkedExample(t *testing.T) {
	b := healthFor(t, fileMetrics{parsed: true, codeLines: 600, imports: 20, complexLines: 75}, "OrderService.java")
	if math.Abs(b.health-4.5) > 1e-9 {
		t.Errorf("health %v, want 4.5", b.health)
	}
}

func TestHealthFloorsAtOne(t *testing.T) {
	b := healthFor(t, fileMetrics{parsed: true, codeLines: 30000, imports: 200, complexLines: 20000}, "a.ts")
	if b.health != 1 {
		t.Errorf("health %v, want 1", b.health)
	}
}

// Indentation no longer costs a parsed file anything: deep nesting is read
// from its functions.
func TestIndentationDoesNotCostAParsedFile(t *testing.T) {
	b := healthFor(t, fileMetrics{parsed: true, codeLines: 100, maxIndentation: 12, avgIndentation: 4, deepLines: 80}, "a.go")
	if b.health != 10 {
		t.Errorf("health %v, want 10", b.health)
	}
}

// A language no pack parses is read by its indentation: size from its
// non-blank lines, deep code from the lines three levels in.
func TestHealthFallsBackToIndentation(t *testing.T) {
	b := healthFor(t, fileMetrics{nonBlank: 360, deepLines: 75}, "src/lib.rs")
	if !b.fallback || math.Abs(b.size-1) > 1e-9 || math.Abs(b.deepCode-2) > 1e-9 || b.coupling != 0 || math.Abs(b.health-7) > 1e-9 {
		t.Errorf("breakdown %+v", b)
	}
}

// Markup, data and scripts with no pack get no reading.
func TestNoHealthForTextThatIsNotAProgrammingLanguage(t *testing.T) {
	for _, p := range []string{"index.html", "schema.sql", "deploy.sh", "docs/page.mdx", "pom.xml"} {
		if _, ok := healthOf(fileMetrics{nonBlank: 900, deepLines: 300}, p); ok {
			t.Errorf("%s: want no health", p)
		}
	}
}

func TestCalculateBumpyRoad(t *testing.T) {
	// Volatility = 20, lines = 100 -> bumpy road density = 0.2
	m := fileMetrics{lines: 100, volatility: 20}
	bumpy := calculateBumpyRoad(m)
	if bumpy != 0.2 {
		t.Errorf("Expected bumpy road density 0.2, got %f", bumpy)
	}

	// Empty file
	m = fileMetrics{lines: 0, volatility: 20}
	if calculateBumpyRoad(m) != 0.0 {
		t.Error("Expected 0.0 bumpy road for zero lines")
	}
}

func TestCalculateStaticComplexity(t *testing.T) {
	// sc = lines * (1 + avgIndentation) = 100 * (1 + 2.5) = 350.0
	m := fileMetrics{lines: 100, avgIndentation: 2.5}
	sc := calculateStaticComplexity(m)
	if sc != 350.0 {
		t.Errorf("Expected 350.0, got %f", sc)
	}

	// Capping at 8.0: sc = 100 * (1 + 8) = 900.0
	m = fileMetrics{lines: 100, avgIndentation: 12.0}
	sc = calculateStaticComplexity(m)
	if sc != 900.0 {
		t.Errorf("Expected 900.0, got %f", sc)
	}
}

func TestCalculateHotspotScore(t *testing.T) {
	// Normal case
	score := calculateHotspotScore(5000, 10000, true)
	if score != 50.0 {
		t.Errorf("Expected 50.0, got %f", score)
	}

	// Max file
	score = calculateHotspotScore(10000, 10000, true)
	if score != 100.0 {
		t.Errorf("Expected 100.0, got %f", score)
	}

	// No commits
	score = calculateHotspotScore(0, 10000, false)
	if score != 0.0 {
		t.Errorf("Expected 0.0, got %f", score)
	}

	// Zero max
	score = calculateHotspotScore(100, 0, true)
	if score != 0.0 {
		t.Errorf("Expected 0.0 (zero max), got %f", score)
	}
}

func TestExtractFileMetrics(t *testing.T) {
	s := stats.Stats{
		"complexity__lines":                   100,
		"complexity__indentation__max":        5,
		"complexity__indentation__avg":        2.5,
		"complexity__indentation__volatility": 12,
		"git__commits__total":                 42,
	}
	m := extractFileMetrics(&s)
	if m.lines != 100 {
		t.Errorf("Expected lines=100, got %d", m.lines)
	}
	if m.maxIndentation != 5 {
		t.Errorf("Expected maxIndentation=5, got %d", m.maxIndentation)
	}
	if m.avgIndentation != 2.5 {
		t.Errorf("Expected avgIndentation=2.5, got %f", m.avgIndentation)
	}
	if m.commits != 42 {
		t.Errorf("Expected commits=42, got %d", m.commits)
	}
	if m.volatility != 12 {
		t.Errorf("Expected volatility=12, got %d", m.volatility)
	}
}

func TestExtractFileMetrics_NoGit(t *testing.T) {
	s := stats.Stats{
		"complexity__lines":            200,
		"complexity__indentation__max": 3,
		"complexity__indentation__avg": 1.0,
	}
	m := extractFileMetrics(&s)
	if m.commits != 0 {
		t.Errorf("Expected commits=0 when git missing, got %d", m.commits)
	}
	if m.lines != 200 {
		t.Errorf("Expected lines=200, got %d", m.lines)
	}
}

func TestAverageAccumulator(t *testing.T) {
	avg := averageAccumulator([]interface{}{8.0, 4.0, 6.0})
	if avg != 6.0 {
		t.Errorf("Expected 6.0, got %v", avg)
	}

	avg = averageAccumulator([]interface{}{})
	if avg != 0.0 {
		t.Errorf("Expected 0.0 for empty, got %v", avg)
	}
}

func TestSumAccumulator(t *testing.T) {
	// Float values
	sumVal := sumAccumulator([]interface{}{1.0, 2.0, 3.5})
	if sumVal != 6.5 {
		t.Errorf("Expected 6.5, got %v", sumVal)
	}

	// Int values
	sumInt := sumAccumulator([]interface{}{1, 2, 3})
	if sumInt != 6 {
		t.Errorf("Expected 6, got %v", sumInt)
	}

	// Mixed
	sumMixed := sumAccumulator([]interface{}{1, 2.5, 3})
	if sumMixed != 6.5 {
		t.Errorf("Expected 6.5, got %v", sumMixed)
	}
}

// Text that is not code gets no health reading: a translation catalogue and
// a stylesheet topped the hotspot lists of django-oscar and LibreChat.
func TestNonCodeTextGetsNoCodeSmells(t *testing.T) {
	for _, p := range []string{
		"src/oscar/locale/fr/LC_MESSAGES/django.po",
		"client/src/style.css",
		"themes/admin/scss/_layout.scss",
		"static/icons/logo.svg",
		"dist-maps/app.js.map",
	} {
		if !isExcludedFromCodeSmells(p) {
			t.Errorf("%s: want excluded", p)
		}
	}
	for _, p := range []string{"src/oscar/apps/basket/models.py", "client/src/App.tsx", "Nop.Core/Caching/CacheKey.cs"} {
		if isExcludedFromCodeSmells(p) {
			t.Errorf("%s: want a reading", p)
		}
	}
}

// A stored breakdown always reproduces its score.
func TestHealthIsTenLessItsDeductions(t *testing.T) {
	for _, m := range []fileMetrics{
		{parsed: true, codeLines: 120, imports: 3},
		{parsed: true, codeLines: 900, imports: 25, complexLines: 140},
		{nonBlank: 4000, deepLines: 900},
	} {
		b := healthFor(t, m, map[bool]string{true: "a.go", false: "a.rb"}[m.parsed])
		want := math.Max(1, 10-b.size-b.coupling-b.complexCode-b.deepCode)
		if math.Abs(b.health-want) > 1e-9 {
			t.Errorf("%+v: health %v, deductions give %v", m, b.health, want)
		}
	}
}
