package util

import (
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/stats"
)

// A component never carries a file's deductions or thresholds, even when it
// has one file; its raw hotspot is its hottest file's.
func TestRollUpDropsFileOnlyReadings(t *testing.T) {
	// No file records to recompute from: the group's own readings are what
	// is being checked.
	results := &core.Results{StatRecordsByFile: map[string][]*stats.Record{}}
	group := stats.Stats{"codesmells__health__threshold__max_nesting": 4, "codesmells__health__deduction__size": 0.0}
	RollUpCodeSmells(results, []string{"a.go"}, &group)
	for _, k := range []string{"codesmells__health__threshold__max_nesting", "codesmells__health__deduction__size"} {
		if _, ok := group[k]; ok {
			t.Errorf("group kept %s", k)
		}
	}
}
