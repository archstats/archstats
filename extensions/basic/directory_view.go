package basic

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/extensions/util"
	"github.com/samber/lo"
)

func getStatsByDirectory(results *core.Results) map[string]*stats.Stats {
	statsByDirectory := lo.MapValues(results.DirectoryToFiles, func(files []string, component string) *stats.Stats {
		var stats_ []*stats.Record
		for _, file := range files {
			stats_ = append(stats_, results.StatRecordsByFile[file]...)
		}
		stats_ = append(stats_, &stats.Record{
			StatType: file.FileCount,
			Value:    len(files),
		})
		return results.Calculate(stats_)
	})
	return statsByDirectory
}
func directoryView(results *core.Results) *core.View {
	statsByDirectory := getStatsByDirectory(results)
	return util.GenericView(util.GetDistinctColumnsFrom(statsByDirectory), statsByDirectory)
}
