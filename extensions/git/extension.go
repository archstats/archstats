package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
	"slices"
	"strings"
	"time"
)

const (
	AuthorCount                = "author_count"
	AgeInDays                  = "age_in_days"
	File                       = "file"
	Component                  = "component"
	CommitHash                 = "commit_hash"
	CommitTime                 = "commit_time"
	AuthorName                 = "author_name"
	AuthorEmail                = "author_email"
	CommitMessage              = "commit_message"
	CommitFileAdditions        = "file_additions"
	CommitFileDeletions        = "file_deletions"
	CommitCount                = "commit_count"
	AdditionCount              = "addition_count"
	DeletionCount              = "deletion_count"
	UniqueFileChangeCount      = "unique_file_change_count"
	UniqueComponentChangeCount = "unique_component_change_count"
)

// TODO
// TESTS.............. you're better than this, Ryan.
//
// Per file and author combination:
// Number of additions
// Number of deletions
// Number of commits

func Extension() core.Extension {
	return &extension{
		DayBuckets:                           []int{30, 90, 180},
		GenerateCommitView:                   true,
		GenerateFileLogicalCouplingView:      true,
		GenerateComponentLogicalCouplingView: true,
		GitAfter:                             "",
		GitSince:                             "",
		BasedOn:                              time.Now(),
	}
}

type extension struct {
	// The number of days to bucket stats into. For example, if this is set to [7, 30, 90], then the stats will be bucketed into
	// 7 days, 30 days, and 90 days. This is useful for seeing code churn over time.
	DayBuckets []int

	// If true, a view will be generated that shows all commits
	GenerateCommitView bool

	// If true, a view will be generated that shows the logical coupling between files
	GenerateFileLogicalCouplingView bool

	// If true, a view will be generated that shows the logical coupling between components
	GenerateComponentLogicalCouplingView bool

	// Passed to `git log --after`
	GitAfter string
	// Passed to `git log --since`
	GitSince string

	// What is the base date for the stats? If this is set, the aggregated time stats will be relative to this date.
	BasedOn time.Time

	// Represents an individual change in a commit. A commit can have multiple parts if it changes multiple files.
	commitParts []*commits.PartOfCommit

	splittedCommits *commits.Splitted
}

func (e *extension) Init(settings core.Analyzer) error {
	settings.RegisterResultsEditor(e)
	settings.RegisterView(&core.ViewFactory{
		Name:           "git_authors",
		CreateViewFunc: e.authorViewFactory,
	})

	//if e.GenerateComponentLogicalCouplingView {
	//	settings.RegisterView(&core.ViewFactory{
	//		Name:           "git_component_logical_coupling",
	//		CreateViewFunc: e.componentCouplingViewFactory,
	//	})
	//}
	//
	//if e.GenerateFileLogicalCouplingView {
	//	settings.RegisterView(&core.ViewFactory{
	//		Name:           "git_file_logical_coupling",
	//		CreateViewFunc: e.fileCouplingViewFactory,
	//	})
	//}

	if e.GenerateCommitView {
		settings.RegisterView(&core.ViewFactory{
			Name:           "git_commits",
			CreateViewFunc: e.commitViewFactory,
		})
	}

	rawCommits, err := e.parseGitLog(settings.RootPath())
	if err != nil {
		return err
	}

	e.commitParts = lo.FlatMap(rawCommits, func(commit *rawCommit, index int) []*commits.PartOfCommit {
		return gitCommitToPartOfCommit(commit)
	})

	return nil
}

// TODO add definitions after API is stable
func (e *extension) definitions() []*definitions.Definition {

	return []*definitions.Definition{}
}

func (e *extension) EditResults(results *core.Results) {
	setComponent(results, e.commitParts)

	e.splittedCommits = commits.Split(e.BasedOn, e.DayBuckets, e.commitParts)

	setStatsTotal(e.BasedOn, results.StatsByFile, e.splittedCommits.SplitByFile())
	setStatsTotal(e.BasedOn, results.StatsByComponent, e.splittedCommits.SplitByComponent())

	for days, split := range e.splittedCommits.DayBuckets() {

		setStatsLastXDays(e.BasedOn, days, results.StatsByFile, split.SplitByFile())
		setStatsLastXDays(e.BasedOn, days, results.StatsByComponent, split.SplitByComponent())
	}
}

func setComponent(results *core.Results, commitParts []*commits.PartOfCommit) {
	for _, part := range commitParts {
		part.Component = results.FileToComponent[part.File]
	}
}

func gitCommitToPartOfCommit(rawCommit *rawCommit) []*commits.PartOfCommit {
	return lo.Map(rawCommit.Files, func(file *rawPartOfCommit, _ int) *commits.PartOfCommit {
		return &commits.PartOfCommit{
			Component:   "",
			Commit:      rawCommit.Hash,
			Time:        rawCommit.Time,
			File:        file.Path,
			Author:      rawCommit.AuthorName,
			AuthorEmail: rawCommit.AuthorEmail,
			Message:     rawCommit.Message,
			Additions:   file.Additions,
			Deletions:   file.Deletions,
		}
	})
}

func setStatsTotal(basedOn time.Time, statGroup file.StatsGroup, group map[string][]*commits.PartOfCommit) {
	for filePath, _ := range statGroup {
		commitParts := group[filePath]

		stats := commits.GetStats(basedOn, commitParts)
		statGroup.SetStat(filePath, AdditionCount, stats.AdditionCount)
		statGroup.SetStat(filePath, DeletionCount, stats.DeletionCount)
		statGroup.SetStat(filePath, CommitCount, stats.CommitCount)
		statGroup.SetStat(filePath, AuthorCount, stats.UniqueAuthorCount)
		statGroup.SetStat(filePath, AgeInDays, stats.OldestCommitAgeInDays)
		statGroup.SetStat(filePath, UniqueFileChangeCount, stats.UniqueFileChangeCount)
	}
}

func setStatsLastXDays(basedOn time.Time, days int, statGroup file.StatsGroup, group map[string][]*commits.PartOfCommit) {
	for filePath, _ := range group {
		commitParts := group[filePath]

		stats := commits.GetStats(basedOn, commitParts)
		statGroup.SetStat(filePath, toDayStat(AdditionCount, days), stats.AdditionCount)
		statGroup.SetStat(filePath, toDayStat(DeletionCount, days), stats.DeletionCount)
		statGroup.SetStat(filePath, toDayStat(CommitCount, days), stats.CommitCount)
		statGroup.SetStat(filePath, toDayStat(AuthorCount, days), stats.UniqueAuthorCount)
		statGroup.SetStat(filePath, toDayStat(UniqueFileChangeCount, days), stats.UniqueFileChangeCount)
	}
}

func sharedCommitColumns(dayBuckets []int) []*core.Column {
	columns := []*core.Column{
		core.StringColumn("from"),
		core.StringColumn("to"),
		core.IntColumn("shared_commit_count"),
	}

	for _, days := range dayBuckets {
		columns = append(columns, core.IntColumn(toDayStat("shared_commit_count", days)))
	}
	return columns
}

// Takes total shared commit counts and shared commit counts per day bucket and returns rows.
func sharedCommitsToRows(componentsOrFiles []string, totals map[string]commits.CommitHashes, totalsPerDayBucket map[int]map[string]commits.CommitHashes) []*core.Row {
	var rows []*core.Row
	for _, component1 := range componentsOrFiles {
		for _, component2 := range componentsOrFiles {
			if component1 == component2 {
				continue
			}

			combined := []string{component1, component2}
			slices.Sort(combined)
			key := strings.Join(combined, ":")

			row1 := &core.Row{
				Data: map[string]interface{}{
					"from": component1,
					"to":   component2,
				},
			}
			row2 := &core.Row{
				Data: map[string]interface{}{
					"from": component2,
					"to":   component1,
				},
			}

			if total, hasTotal := totals[key]; hasTotal {
				row1.Data["shared_commit_count"] = total
				row2.Data["shared_commit_count"] = total
			}

			for days, sharedCommitCount := range totalsPerDayBucket {
				if count, hasCount := sharedCommitCount[key]; hasCount {
					row1.Data[toDayStat("shared_commit_count", days)] = count
					row2.Data[toDayStat("shared_commit_count", days)] = count
				}
			}

			rows = append(rows, row1, row2)
		}
	}
	return rows
}
