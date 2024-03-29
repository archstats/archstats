package file

import (
	"github.com/samber/lo"
)

// Stats are a map of stat type to value
// For example: {"function": 10, "class": 5}
// These stats are usually the number of Snippet of a certain type
// Stats can also be generated by a StatProvider
type Stats map[string]interface{}

// StatsGroup is a mapping of a name to a Stats
type StatsGroup map[string]*Stats

func (s StatsGroup) SetStat(key, stat string, value interface{}) {
	if _, has := s[key]; !has {
		s[key] = &Stats{}
	}
	(*s[key])[stat] = value
}

func SnippetsToStats(snippets []*Snippet) []*StatRecord {
	stats := make(map[string]int)
	for _, snippet := range snippets {
		statName := snippet.Type
		stats[statName]++
	}

	return lo.MapToSlice(stats, func(key string, value int) *StatRecord {
		return &StatRecord{StatType: key, Value: value}
	})
}
