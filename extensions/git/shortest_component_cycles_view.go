package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) shortestCycleViewFactory(results *core.Results) *core.View {

	graph := results.ComponentGraph
	cycles := graph.ShortestCycles()
	hashIdx := e.splittedCommits.SplitByCommitHash()

	var rows []*core.Row
	for cycleKey, cycle := range cycles {
		commitHashesInCommon := commits.SharedCommitsForGroup(cycle, e.splittedCommits.ComponentToCommitHashes())

		commitsInCommon := lo.FlatMap(commitHashesInCommon, func(commitHash string, _ int) []*commits.PartOfCommit {
			return hashIdx[commitHash]
		})

		splittedCommonCommits := commits.Split(e.BasedOn, e.DayBuckets, commitsInCommon)

		row := &core.Row{
			Data: map[string]interface{}{
				"cycle":           cycleKey,
				"cycle_size":      len(cycle) - 1,
				SharedCommitCount: len(commitHashesInCommon),
			},
		}

		for days, splitted := range splittedCommonCommits.DayBuckets() {
			hashes := splitted.SplitByCommitHash()
			row.Data[toDayStat(SharedCommitCount, days)] = len(hashes)
		}

		rows = append(rows, row)
	}

	columns := []*core.Column{
		core.StringColumn("cycle"),
		core.IntColumn("cycle_size"),
		core.IntColumn(SharedCommitCount),
	}

	for _, days := range e.DayBuckets {
		columns = append(columns, core.IntColumn(toDayStat(SharedCommitCount, days)))
	}
	return &core.View{
		Name:    "git_component_cycles_shortest_shared_commits",
		Columns: columns,
		Rows:    rows,
	}
}
