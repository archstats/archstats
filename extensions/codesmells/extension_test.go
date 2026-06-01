package codesmells

import (
	"github.com/archstats/archstats/core/stats"
	"testing"
)

func TestCalculateCodeHealth_PerfectFile(t *testing.T) {
	m := fileMetrics{lines: 100, maxIndentation: 2, avgIndentation: 1.0}
	health := calculateCodeHealth(m)
	if health != 10.0 {
		t.Errorf("Expected 10.0, got %f", health)
	}
}

func TestCalculateCodeHealth_GodFile(t *testing.T) {
	// 800 lines: deduction = (800 - 500) * 0.01 = 3.0 (capped)
	m := fileMetrics{lines: 800, maxIndentation: 2, avgIndentation: 1.0}
	health := calculateCodeHealth(m)
	if health != 7.0 {
		t.Errorf("Expected 7.0, got %f", health)
	}
}

func TestCalculateCodeHealth_DeepNesting(t *testing.T) {
	// max indent 8: deduction = (8 - 4) * 0.5 = 2.0
	m := fileMetrics{lines: 100, maxIndentation: 8, avgIndentation: 1.0}
	health := calculateCodeHealth(m)
	if health != 8.0 {
		t.Errorf("Expected 8.0, got %f", health)
	}
}

func TestCalculateCodeHealth_HighAvgNesting(t *testing.T) {
	// avg indent 3.5: deduction = (3.5 - 1.5) * 1.5 = 3.0 (capped)
	m := fileMetrics{lines: 100, maxIndentation: 2, avgIndentation: 3.5}
	health := calculateCodeHealth(m)
	if health != 7.0 {
		t.Errorf("Expected 7.0, got %f", health)
	}
}

func TestCalculateCodeHealth_AllDeductions(t *testing.T) {
	// God File: (1000 - 500) * 0.01 = 5.0, capped at 3.0
	// Max nesting: (8 - 4) * 0.5 = 2.0
	// Avg nesting: (3.5 - 1.5) * 1.5 = 3.0
	// Total deductions = 3.0 + 2.0 + 3.0 = 8.0
	// health = 10.0 - 8.0 = 2.0
	m := fileMetrics{lines: 1000, maxIndentation: 8, avgIndentation: 3.5}
	health := calculateCodeHealth(m)
	if health != 2.0 {
		t.Errorf("Expected 2.0, got %f", health)
	}
}

func TestCalculateCodeHealth_FloorAtOne(t *testing.T) {
	// Extreme values that would push below 1.0
	// God file: capped at 3.0
	// Max indent 100: (100-4)*0.5 = 48.0, capped at 3.0
	// Avg indent 100.0: (100.0-1.5)*1.5 = 147.75, capped at 3.0
	// Total = 9.0 → health = 1.0
	m := fileMetrics{lines: 10000, maxIndentation: 100, avgIndentation: 100.0}
	health := calculateCodeHealth(m)
	if health != 1.0 {
		t.Errorf("Expected 1.0, got %f", health)
	}
}

func TestCalculateBumpyRoad(t *testing.T) {
	// Both conditions met
	m := fileMetrics{maxIndentation: 6, avgIndentation: 2.0}
	if calculateBumpyRoad(m) != 1 {
		t.Error("Expected bumpy road = 1")
	}

	// Only max indentation high
	m = fileMetrics{maxIndentation: 6, avgIndentation: 1.0}
	if calculateBumpyRoad(m) != 0 {
		t.Error("Expected bumpy road = 0 (avg too low)")
	}

	// Only avg indentation high
	m = fileMetrics{maxIndentation: 3, avgIndentation: 2.0}
	if calculateBumpyRoad(m) != 0 {
		t.Error("Expected bumpy road = 0 (max too low)")
	}

	// Neither condition
	m = fileMetrics{maxIndentation: 3, avgIndentation: 1.0}
	if calculateBumpyRoad(m) != 0 {
		t.Error("Expected bumpy road = 0")
	}
}

func TestCalculateStaticComplexity(t *testing.T) {
	m := fileMetrics{lines: 100, maxIndentation: 5}
	sc := calculateStaticComplexity(m)
	if sc != 500.0 {
		t.Errorf("Expected 500.0, got %f", sc)
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
		"complexity__lines":            100,
		"complexity__indentation__max": 5,
		"complexity__indentation__avg": 2.5,
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
