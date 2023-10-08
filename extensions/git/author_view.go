package git

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
	"strconv"
	"time"
)

func (e *extension) authorViewFactory(*core.Results) *core.View {
	commitPartsByAuthor := splitByAuthor(e.commitParts)
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

func getAuthorRowStats(basedOn time.Time, author string, commitParts []*partOfCommit, buckets []int) *core.Row {

	rowData := map[string]interface{}{}

	allStats := getCommitStats(basedOn, commitParts)
	rowData[AuthorName] = author
	rowData[AuthorEmail] = commitParts[0].authorEmail

	rowData[AdditionCount] = allStats.additionCount
	rowData[DeletionCount] = allStats.deletionCount
	rowData[CommitCount] = allStats.commitCount
	rowData[UniqueFileChangeCount] = allStats.uniqueFileChangeCount
	rowData[UniqueComponentChangeCount] = allStats.uniqueComponentChangeCount

	bucketsMap := splitCommitsIntoBucketsOfDays(commitParts[0].time, commitParts, buckets)
	for days, bucket := range bucketsMap {
		stats := getCommitStats(basedOn, bucket)

		rowData[toDayStat(AdditionCount, days)] = stats.additionCount
		rowData[toDayStat(DeletionCount, days)] = stats.deletionCount
		rowData[toDayStat(CommitCount, days)] = stats.commitCount
		rowData[toDayStat(UniqueFileChangeCount, days)] = stats.uniqueFileChangeCount
		rowData[toDayStat(UniqueComponentChangeCount, days)] = stats.uniqueComponentChangeCount
	}

	return &core.Row{
		Data: rowData,
	}
}

func toDayStat(stat string, days int) string {
	return stat + ":" + strconv.Itoa(days) + "_days"
}
