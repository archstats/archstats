package codesmells

import (
	"embed"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/extensions/indentations"
	"github.com/archstats/archstats/extensions/treesitter/common"
	"math"
	"strings"
)

const (
	CodeHealth            = "codesmells__code_health"
	HotspotScore          = "codesmells__hotspot_score"
	BumpyRoad             = "codesmells__bumpy_road"
	StaticComplexityScore = "codesmells__static_complexity_score"

	// The deductions behind a file's health score, so a reader can see why
	// it scored what it did. File-only: summed over a component they mean
	// nothing, so components and directories carry none. A file read by its
	// indentation has a deep-code deduction instead of complex code and
	// coupling.
	DeductionSize        = "codesmells__health__deduction__size"
	DeductionCoupling    = "codesmells__health__deduction__coupling"
	DeductionComplexCode = "codesmells__health__deduction__complex_code"
	DeductionDeepCode    = "codesmells__health__deduction__deep_code"
	// HealthWorstFile is a group's least healthy file, shown beside its
	// line-weighted health.
	HealthWorstFile = "codesmells__code_health__worst_file"
	// HotspotRaw is log2(commits + 1) × lines before normalising to 0–100
	// against the hottest file; rolled up as the hottest file's.
	HotspotRaw = "codesmells__hotspot__raw"
)

// FileOnlyReadings explain one file's score; a group carries none of them.
var FileOnlyReadings = []string{DeductionSize, DeductionCoupling, DeductionComplexCode, DeductionDeepCode}

func Extension() core.Extension {
	return &extension{}
}

type extension struct{}

//go:embed definitions/**
var defs embed.FS

func (e *extension) Init(settings core.Analyzer) error {
	loadedDefs, err := definitions.LoadYamlFiles(defs)
	if err != nil {
		return err
	}

	for _, definition := range loadedDefs {
		settings.AddDefinition(definition)
	}

	// Register results editor
	settings.RegisterResultsEditor(e)

	// Register accumulators
	settings.RegisterStatAccumulator(CodeHealth, averageAccumulator)
	settings.RegisterStatAccumulator(HotspotScore, averageAccumulator)
	settings.RegisterStatAccumulator(BumpyRoad, averageAccumulator)
	settings.RegisterStatAccumulator(StaticComplexityScore, sumAccumulator)
	for _, fileOnly := range FileOnlyReadings {
		settings.RegisterStatAccumulator(fileOnly, onlyOneAccumulator)
	}
	settings.RegisterStatAccumulator(HotspotRaw, maxAccumulator)
	settings.RegisterStatAccumulator(HealthWorstFile, minAccumulator)
	settings.RegisterStatAccumulator(common.CognitiveMax, maxAccumulator)
	settings.RegisterView(&core.ViewFactory{Name: "functions", CreateViewFunc: functionsView})
	settings.RegisterView(&core.ViewFactory{Name: "complexity_increments", CreateViewFunc: complexityIncrementsView})

	return nil
}

// fileMetrics holds the raw inputs extracted from a file's stats.
type fileMetrics struct {
	lines          int
	commits        int
	maxIndentation int
	avgIndentation float64
	volatility     int

	// parsed is whether a language pack measured the file's functions, and
	// with them its code lines, complex lines and imports.
	parsed       bool
	codeLines    int
	complexLines int
	imports      int
	// The indentation reader's, for a file no pack parsed.
	nonBlank, deepLines int
}

// calculatedMetrics holds the computed codesmell outputs for a single file.
type calculatedMetrics struct {
	codeHealth            float64
	hotspotScore          float64
	bumpyRoad             float64
	staticComplexityScore float64
}

// extractFileMetrics pulls the relevant raw stats from a Stats map.
func extractFileMetrics(fileStats *stats.Stats) fileMetrics {
	m := fileMetrics{}
	if fileStats == nil {
		return m
	}
	if val, exists := (*fileStats)["complexity__lines"]; exists {
		if i, ok := val.(int); ok {
			m.lines = i
		}
	}
	if val, exists := (*fileStats)["git__commits__total"]; exists {
		if i, ok := val.(int); ok {
			m.commits = i
		}
	}
	if val, exists := (*fileStats)["complexity__indentation__max"]; exists {
		if i, ok := val.(int); ok {
			m.maxIndentation = i
		}
	}
	if val, exists := (*fileStats)["complexity__indentation__avg"]; exists {
		if f, ok := val.(float64); ok {
			m.avgIndentation = f
		} else if i, ok := val.(int); ok {
			m.avgIndentation = float64(i)
		}
	}
	if val, exists := (*fileStats)["complexity__indentation__volatility"]; exists {
		if i, ok := val.(int); ok {
			m.volatility = i
		}
	}
	if val, exists := (*fileStats)[common.CodeLines]; exists {
		m.parsed = true
		m.codeLines = intOf(val)
		m.complexLines = intOf((*fileStats)[common.ComplexLines])
		m.imports = intOf((*fileStats)[common.Imports])
	}
	m.nonBlank = intOf((*fileStats)[indentations.NonBlank])
	m.deepLines = intOf((*fileStats)[indentations.Deep])
	return m
}

func intOf(v interface{}) int {
	f, _ := asFloat(v)
	return int(f)
}

// isExcludedFromCodeSmells identifies non-code/configuration files to ignore.
func isExcludedFromCodeSmells(path string) bool {
	lowerPath := strings.ToLower(path)

	// Check standard exclusions
	exclusions := []string{
		"node_modules/",
		"vendor/",
		"dist/",
		"build/",
		"target/",
		".git/",
		".github/",
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"go.sum",
		"cargo.lock",
		"composer.lock",
	}
	for _, excl := range exclusions {
		// A directory, not a package named like one: see InDirOutsideSourceRoot.
		if strings.HasSuffix(excl, "/") {
			if file.InDirOutsideSourceRoot(lowerPath, excl) {
				return true
			}
			continue
		}
		if strings.Contains(lowerPath, excl) {
			return true
		}
	}

	// Text that is not code gets no health reading: a translation catalogue
	// and a stylesheet topped the hotspot lists of django-oscar and
	// LibreChat. The list is core's, shared with file roles.
	if file.IsNonCode(lowerPath) {
		return true
	}

	return false
}

// calculateBumpyRoad returns the volatility per non-empty line of code.
func calculateBumpyRoad(m fileMetrics) float64 {
	if m.lines == 0 {
		return 0.0
	}
	return float64(m.volatility) / float64(m.lines)
}

// calculateStaticComplexity returns lines * (1 + avgIndentation) with capped avgIndentation.
func calculateStaticComplexity(m fileMetrics) float64 {
	avg := m.avgIndentation
	if avg > 8.0 {
		avg = 8.0
	}
	return float64(m.lines) * (1.0 + avg)
}

// calculateHotspotScore normalises the raw hotspot, log2(commits + 1) × lines,
// to 0–100 against the hottest file in the snapshot.
func calculateHotspotScore(rawHotspot, maxRawHotspot float64, hasCommits bool) float64 {
	if !hasCommits || maxRawHotspot <= 0 {
		return 0.0
	}
	return (rawHotspot * 100.0) / maxRawHotspot
}

func (e *extension) EditResults(results *core.Results) {
	// First pass: extract metrics and compute raw hotspots
	type fileInfo struct {
		metrics    fileMetrics
		rawHotspot float64
	}
	files := make(map[string]*fileInfo)
	var maxRawHotspot float64

	for file, records := range results.StatRecordsByFile {
		// Vendored and minified code is not the project's to keep healthy, and
		// was the hottest "hotspot" in at least one codebase.
		if isExcludedFromCodeSmells(file) || results.ThirdPartyFiles[file] || results.GeneratedFiles[file] {
			continue // skip excluded files entirely from codesmell evaluations
		}
		fileStats := results.Calculate(records)
		m := extractFileMetrics(fileStats)

		// Log-scale commits: rawHotspot = log2(commits + 1) * lines
		raw := math.Log2(float64(m.commits)+1.0) * float64(m.lines)
		files[file] = &fileInfo{metrics: m, rawHotspot: raw}
		if raw > maxRawHotspot {
			maxRawHotspot = raw
		}
	}

	// Second pass: compute final metrics and append records
	for file, info := range files {
		m := info.metrics
		records := []*stats.Record{
			{StatType: HotspotScore, Value: calculateHotspotScore(info.rawHotspot, maxRawHotspot, m.commits > 0)},
			{StatType: BumpyRoad, Value: calculateBumpyRoad(m)},
			{StatType: StaticComplexityScore, Value: calculateStaticComplexity(m)},
			{StatType: HotspotRaw, Value: info.rawHotspot},
		}
		if b, ok := healthOf(m, file); ok {
			records = append(records,
				&stats.Record{StatType: CodeHealth, Value: b.health},
				&stats.Record{StatType: DeductionSize, Value: b.size},
			)
			if b.fallback {
				records = append(records, &stats.Record{StatType: DeductionDeepCode, Value: b.deepCode})
			} else {
				records = append(records,
					&stats.Record{StatType: DeductionCoupling, Value: b.coupling},
					&stats.Record{StatType: DeductionComplexCode, Value: b.complexCode},
				)
			}
		}
		results.StatRecordsByFile[file] = append(results.StatRecordsByFile[file], records...)
	}
}

func (e *extension) typeAssertions() (core.Extension, core.ResultsEditor) {
	return e, e
}

// Accumulator mergers
func averageAccumulator(values []interface{}) interface{} {
	if len(values) == 0 {
		return 0.0
	}
	var sum float64
	var count int
	for _, val := range values {
		if val == nil {
			continue
		}
		if f, ok := val.(float64); ok {
			sum += f
			count++
		} else if i, ok := val.(int); ok {
			sum += float64(i)
			count++
		}
	}
	if count == 0 {
		return 0.0
	}
	return sum / float64(count)
}

func sumAccumulator(values []interface{}) interface{} {
	var sum float64
	var isFloat bool
	for _, val := range values {
		if val == nil {
			continue
		}
		if f, ok := val.(float64); ok {
			sum += f
			isFloat = true
		} else if i, ok := val.(int); ok {
			sum += float64(i)
		}
	}
	if isFloat {
		return sum
	}
	return int(sum)
}

// onlyOneAccumulator keeps a value that belongs to one file: over a group it
// is nothing, not a sum of thresholds.
func onlyOneAccumulator(values []interface{}) interface{} {
	if len(values) == 1 {
		return values[0]
	}
	return nil
}

func minAccumulator(values []interface{}) interface{} {
	var best float64
	found := false
	for _, v := range values {
		f, ok := asFloat(v)
		if ok && (!found || f < best) {
			best, found = f, true
		}
	}
	if !found {
		return nil
	}
	return best
}

func maxAccumulator(values []interface{}) interface{} {
	var best float64
	found := false
	for _, v := range values {
		f, ok := asFloat(v)
		if ok && (!found || f > best) {
			best, found = f, true
		}
	}
	if !found {
		return nil
	}
	return best
}

func asFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
