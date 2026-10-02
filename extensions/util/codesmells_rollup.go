package util

import (
	"math"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/stats"
)

// RollUpCodeSmells replaces the plain-average code-health readings of a group
// of files with the size-aware ones: health weighted by code lines (lines
// where no pack measured code lines) beside the group's least healthy file,
// bumpy road weighted by lines, hotspot as the group's hottest file, static
// complexity summed.
//
// Components always did this and directories did not, so the Hotspots view
// read "code health" and "hotspot score" as two different aggregations under
// one label depending on the grain: a directory's hotspot was the MEAN of its
// files, far cooler than the hottest component inside it.
func RollUpCodeSmells(results *core.Results, files []string, groupStats *stats.Stats) {
	// Recalculate codesmells metrics with proper size-weighted logic
	var totalLines float64
	var weightedCodeHealthSum float64
	worstHealth := math.Inf(1)
	var weightedBumpyRoadSum float64
	var bumpyLines float64
	var maxHotspot float64
	var totalStaticComplexity float64

	hasCodeHealth := false
	hasHotspot := false
	hasBumpyRoad := false
	hasStaticComplexity := false

	for _, f := range files {
		fRecords := results.StatRecordsByFile[f]
		if len(fRecords) == 0 {
			continue
		}
		fStats := results.Calculate(fRecords)
		if fStats == nil {
			continue
		}

		var lines float64 = 1.0
		if val, exists := (*fStats)["complexity__lines"]; exists {
			lines = toFloat(val)
			if lines <= 0 {
				lines = 1.0
			}
		}

		if val, exists := (*fStats)["codesmells__code_health"]; exists {
			health := toFloat(val)
			weight := lines
			if code, measured := (*fStats)["complexity__lines__code"]; measured && toFloat(code) > 0 {
				weight = toFloat(code)
			}
			weightedCodeHealthSum += health * weight
			totalLines += weight
			worstHealth = math.Min(worstHealth, health)
			hasCodeHealth = true
		}

		if val, exists := (*fStats)["codesmells__hotspot_score"]; exists {
			hotspot := toFloat(val)
			if hotspot > maxHotspot {
				maxHotspot = hotspot
			}
			hasHotspot = true
		}

		if val, exists := (*fStats)["codesmells__bumpy_road"]; exists {
			bumpy := toFloat(val)
			weightedBumpyRoadSum += bumpy * lines
			bumpyLines += lines
			hasBumpyRoad = true
		}

		if val, exists := (*fStats)["codesmells__static_complexity_score"]; exists {
			complexity := toFloat(val)
			totalStaticComplexity += complexity
			hasStaticComplexity = true
		}
	}

	if hasCodeHealth && totalLines > 0 {
		(*groupStats)["codesmells__code_health"] = weightedCodeHealthSum / totalLines
		(*groupStats)["codesmells__code_health__worst_file"] = worstHealth
	}
	if hasHotspot {
		(*groupStats)["codesmells__hotspot_score"] = maxHotspot
	}
	if hasBumpyRoad && bumpyLines > 0 {
		(*groupStats)["codesmells__bumpy_road"] = weightedBumpyRoadSum / bumpyLines
	}
	if hasStaticComplexity {
		(*groupStats)["codesmells__static_complexity_score"] = totalStaticComplexity
	}
	// Deductions explain one file's score; over a group they are no fact at
	// all, not even for a group of one.
	for _, fileOnly := range []string{
		"codesmells__health__deduction__size", "codesmells__health__deduction__coupling",
		"codesmells__health__deduction__complex_code", "codesmells__health__deduction__deep_code",
	} {
		delete(*groupStats, fileOnly)
	}
}

func toFloat(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0.0
	}
}
