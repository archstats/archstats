package file

import (
	"github.com/archstats/archstats/core/stats"
	"github.com/samber/lo"
)

func SnippetsToStats(snippets []*Snippet) []*stats.Record {
	allStats := make(map[string]int)
	for _, snippet := range snippets {
		statName := snippet.Type
		allStats[statName]++
	}

	return lo.MapToSlice(allStats, func(key string, value int) *stats.Record {
		return &stats.Record{StatType: key, Value: value}
	})
}
