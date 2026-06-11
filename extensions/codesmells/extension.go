package codesmells

import (
	"embed"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/stats"
	"math"
	"path/filepath"
	"strings"
)

const (
	CodeHealth            = "codesmells__code_health"
	HotspotScore          = "codesmells__hotspot_score"
	BumpyRoad             = "codesmells__bumpy_road"
	StaticComplexityScore = "codesmells__static_complexity_score"
)

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

	return nil
}

// fileMetrics holds the raw inputs extracted from a file's stats.
type fileMetrics struct {
	lines          int
	commits        int
	maxIndentation int
	avgIndentation float64
	volatility     int
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
	return m
}

// getLanguageThresholds returns relaxed indentation limits for nested languages.
func getLanguageThresholds(ext string) (maxIndentThreshold int, avgIndentThreshold float64) {
	ext = strings.ToLower(ext)
	switch ext {
	case ".js", ".jsx", ".ts", ".tsx":
		return 6, 2.5 // higher allowance for JavaScript/TypeScript due to callbacks/nested closures
	case ".go":
		return 4, 1.5 // Go standards
	default:
		return 4, 1.5 // general default
	}
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
		if strings.Contains(lowerPath, excl) {
			return true
		}
	}

	// Check file extensions to ignore
	ext := filepath.Ext(lowerPath)
	ignoredExts := map[string]bool{
		".json": true,
		".yaml": true,
		".yml":  true,
		".md":   true,
		".txt":  true,
		".lock": true,
		".xml":  true,
		".toml": true,
		".ini":  true,
		".conf": true,
		".csv":  true,
	}
	if ignoredExts[ext] {
		return true
	}

	return false
}

// calculateCodeHealth computes a 1.0–10.0 code health score.
func calculateCodeHealth(m fileMetrics, ext string) float64 {
	health := 10.0

	// Size deduction (God File): up to 3 points
	if m.lines > 500 {
		deduction := float64(m.lines-500) * 0.01
		if deduction > 3.0 {
			deduction = 3.0
		}
		health -= deduction
	}

	maxIndentThreshold, avgIndentThreshold := getLanguageThresholds(ext)

	// Nesting deduction (Max Indentation): up to 3 points
	if m.maxIndentation > maxIndentThreshold {
		deduction := float64(m.maxIndentation-maxIndentThreshold) * 0.5
		if deduction > 3.0 {
			deduction = 3.0
		}
		health -= deduction
	}

	// Average Nesting deduction: up to 3 points
	if m.avgIndentation > avgIndentThreshold {
		deduction := (m.avgIndentation - avgIndentThreshold) * 1.5
		if deduction > 3.0 {
			deduction = 3.0
		}
		health -= deduction
	}

	if health < 1.0 {
		health = 1.0
	}
	return health
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

// calculateHotspotScore normalises the raw hotspot (commits*lines) to 0–100.
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
		if isExcludedFromCodeSmells(file) {
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
		ext := filepath.Ext(file)
		health := calculateCodeHealth(m, ext)
		bumpyRoadVal := calculateBumpyRoad(m)
		staticComplexity := calculateStaticComplexity(m)
		hotspotVal := calculateHotspotScore(info.rawHotspot, maxRawHotspot, m.commits > 0)

		results.StatRecordsByFile[file] = append(results.StatRecordsByFile[file],
			&stats.Record{StatType: CodeHealth, Value: health},
			&stats.Record{StatType: HotspotScore, Value: hotspotVal},
			&stats.Record{StatType: BumpyRoad, Value: bumpyRoadVal},
			&stats.Record{StatType: StaticComplexityScore, Value: staticComplexity},
		)
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
