package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
	"strconv"
	"time"
)

func (e *extension) authorViewFactory(*core.Results) *core.View {
	commitPartsByAuthor := e.splittedCommits.SplitByAuthor()
	columns := []*core.Column{
		core.StringColumn(AuthorName),
		core.StringColumn(AuthorEmail),
		core.IntColumn(CommitCount),
		core.IntColumn(AdditionCount),
		core.IntColumn(DeletionCount),
		core.IntColumn(UniqueFileChangeCount),
		core.IntColumn(UniqueComponentChangeCount),
	}

	dayBucketColumns := lo.FlatMap(e.DayBuckets, func(days int, _ int) []*core.Column {
		return []*core.Column{
			core.IntColumn(toDayStat(CommitCount, days)),
			core.IntColumn(toDayStat(AdditionCount, days)),
			core.IntColumn(toDayStat(DeletionCount, days)),
			core.IntColumn(toDayStat(UniqueFileChangeCount, days)),
			core.IntColumn(toDayStat(UniqueComponentChangeCount, days)),
		}
	})

	rows := make([]*core.Row, 0)
	for author, commits := range commitPartsByAuthor {
		rows = append(rows, getAuthorRowStats(e.BasedOn, author, commits, e.DayBuckets))
	}
	columns = append(columns, dayBucketColumns...)
	return &core.View{
		Name:    "git_authors",
		Columns: columns,
		Rows:    rows,
	}
}

func getAuthorRowStats(basedOn time.Time, author string, commitParts []*commits.PartOfCommit, buckets []int) *core.Row {

	rowData := map[string]interface{}{}

	allStats := commits.GetStats(basedOn, commitParts)
	rowData[AuthorName] = author
	rowData[AuthorEmail] = commitParts[0].AuthorEmail

	rowData[AdditionCount] = allStats.AdditionCount
	rowData[DeletionCount] = allStats.DeletionCount
	rowData[CommitCount] = allStats.CommitCount
	rowData[UniqueFileChangeCount] = allStats.UniqueFileChangeCount
	rowData[UniqueComponentChangeCount] = allStats.UniqueComponentChangeCount

	/// TODO: this is a bit of a hack, but it works for now
	bucketsMap := commits.SplitCommitsIntoBucketsOfDays(basedOn, commitParts, buckets)
	for days, bucket := range bucketsMap {
		stats := commits.GetStats(basedOn, bucket)

		rowData[toDayStat(AdditionCount, days)] = stats.AdditionCount
		rowData[toDayStat(DeletionCount, days)] = stats.DeletionCount
		rowData[toDayStat(CommitCount, days)] = stats.CommitCount
		rowData[toDayStat(UniqueFileChangeCount, days)] = stats.UniqueFileChangeCount
		rowData[toDayStat(UniqueComponentChangeCount, days)] = stats.UniqueComponentChangeCount
	}

	return &core.Row{
		Data: rowData,
	}
}

func toDayStat(stat string, days int) string {
	return stat + ":last_" + strconv.Itoa(days) + "_days"
}
