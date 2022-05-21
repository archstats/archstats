package archstats

import (
	"regexp"
)

type RegexStatPattern struct {
	Name   string
	Regexp *regexp.Regexp
}
type RegexBasedStats struct {
	Patterns []RegexStatPattern
}

func (r *RegexBasedStats) AfterFileProcessing(results *AfterFileProcessingResults) {
	for _, directory := range results.Directories {
		directory.Stats()
	}
}

func (r *RegexBasedStats) VisitFile(file *file, content []byte) {
	stats := Stats{}
	for _, pattern := range r.Patterns {

		matches := pattern.Regexp.FindAll(content, 100000000)
		stats = stats.Merge(Stats{pattern.Name: len(matches)})
	}

	file.AddStats(stats)
}
