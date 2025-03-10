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
	AuthorCount                = "git__authors"
	AgeInDays                  = "git__age_in_days"
	AdditionCount              = "git__additions"
	DeletionCount              = "git__deletions"
	UniqueFileChangeCount      = "git__unique_file_changes"
	UniqueComponentChangeCount = "git__unique_component_changes"
	CommitCount                = "git__commits"

	File                = "file"
	Component           = "component"
	CommitHash          = "commit_hash"
	CommitTime          = "commit_time"
	AuthorName          = "author_name"
	AuthorEmail         = "author_email"
	CommitMessage       = "commit_message"
	CommitFileAdditions = "file_additions"
	CommitFileDeletions = "file_deletions"

	Pair1                       = "pair_1"
	Pair2                       = "pair_2"
	SharedCommitCount           = "shared_commits"
	PercentageOfAllCommitsPair1 = "percentage_of_all_commits_pair_1"
	PercentageOfAllCommitsPair2 = "percentage_of_all_commits_pair_2"
)

// TODO
// TESTS.............. you're better than this, Ryan.
//
// Per file/component and author combination:
// Number of additions
// Number of deletions
// Number of commits

func Extension() core.Extension {
	return &extension{
		DayBuckets:                           []int{30, 90, 180},
		GenerateCommitView:                   true,
		GenerateFileLogicalCouplingView:      true, // Generates a lot of data
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

	if e.GenerateComponentLogicalCouplingView {
		settings.RegisterView(&core.ViewFactory{
			Name:           "git_component_shared_commits",
			CreateViewFunc: e.componentCouplingViewFactory,
		})
		settings.RegisterView(&core.ViewFactory{
			Name:           "git_component_cycles_shortest_shared_commits",
			CreateViewFunc: e.shortestCycleViewFactory,
		})
		settings.RegisterView(&core.ViewFactory{
			Name:           "git_directory_shared_commits",
			CreateViewFunc: e.directoryCouplingViewFactory,
		})
	}

	if e.GenerateCommitView {
		settings.RegisterView(&core.ViewFactory{
			Name:           "git_commits",
			CreateViewFunc: e.commitViewFactory,
		})
	}

	rawCommits, err := getGitCommitsFromAllReposConcurrently(settings.RootPath())
	if err != nil {
		return err
	}

	e.commitParts = lo.FlatMap(rawCommits, func(commit *rawCommit, index int) []*commits.PartOfCommit {
		return gitCommitToPartOfCommit(settings.RootPath(), commit)
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
	setStatsTotal(e.BasedOn, results.StatsByDirectory, e.splittedCommits.SplitByDirectory())
	setStatsTotal(e.BasedOn, results.StatsByComponent, e.splittedCommits.SplitByComponent())

	for days, split := range e.splittedCommits.DayBuckets() {
		setStatsLastXDays(e.BasedOn, days, results.StatsByFile, split.SplitByFile())
		setStatsLastXDays(e.BasedOn, days, results.StatsByDirectory, split.SplitByDirectory())
		setStatsLastXDays(e.BasedOn, days, results.StatsByComponent, split.SplitByComponent())
	}
}

func setComponent(results *core.Results, commitParts []*commits.PartOfCommit) {
	for _, part := range commitParts {
		part.Component = results.FileToComponent[part.File]
	}
}

func gitCommitToPartOfCommit(rootPath string, rawCommit *rawCommit) []*commits.PartOfCommit {
	return lo.Map(rawCommit.Files, func(file *rawPartOfCommit, _ int) *commits.PartOfCommit {
		//substring length of root away from repo path
		pathToRepo := strings.TrimPrefix(rawCommit.Repo, rootPath)
		// remove leading slash
		if strings.HasPrefix(pathToRepo, "/") {
			pathToRepo = pathToRepo[1:]
		}

		//absolutePath := rootPath + "/" + rawCommit.Repo + "/" + file.Path

		return &commits.PartOfCommit{
			Component:   "",
			Repo:        pathToRepo,
			Commit:      rawCommit.Hash,
			Time:        rawCommit.Time,
			File:        pathToRepo + "/" + file.Path,
			Directory:   getDir(file.Path),
			Author:      rawCommit.AuthorName,
			AuthorEmail: rawCommit.AuthorEmail,
			Message:     rawCommit.Message,
			Additions:   file.Additions,
			Deletions:   file.Deletions,
		}
	})
}

func getDir(path string) string {
	if strings.Contains(path, "/") {
		return path[:strings.LastIndex(path, "/")]
	}
	return ""
}

func setStatsTotal(basedOn time.Time, statGroup file.StatsGroup, group map[string][]*commits.PartOfCommit) {
	for filePath, _ := range statGroup {
		commitParts := group[filePath]

		stats := commits.GetStats(basedOn, commitParts)
		statGroup.SetStat(filePath, AgeInDays, stats.OldestCommitAgeInDays)
		statGroup.SetStat(filePath, toTotalStat(AdditionCount), stats.AdditionCount)
		statGroup.SetStat(filePath, toTotalStat(DeletionCount), stats.DeletionCount)
		statGroup.SetStat(filePath, toTotalStat(CommitCount), stats.CommitCount)
		statGroup.SetStat(filePath, toTotalStat(AuthorCount), stats.UniqueAuthorCount)
		statGroup.SetStat(filePath, toTotalStat(UniqueFileChangeCount), stats.UniqueFileChangeCount)
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
		core.StringColumn(Pair1),
		core.StringColumn(Pair2),
		core.IntColumn(SharedCommitCount),
		core.FloatColumn(PercentageOfAllCommitsPair1),
		core.FloatColumn(PercentageOfAllCommitsPair2),
	}

	for _, days := range dayBuckets {
		columns = append(columns, core.IntColumn(toDayStat(SharedCommitCount, days)))
		columns = append(columns, core.FloatColumn(toDayStat(PercentageOfAllCommitsPair1, days)))
		columns = append(columns, core.FloatColumn(toDayStat(PercentageOfAllCommitsPair2, days)))
	}
	return columns
}

// Takes total shared commit counts and shared commit counts per day bucket and returns rows.
func sharedCommitsToRows(
	componentsOrFiles []string,
	pairToCommitsInCommon map[string]commits.CommitHashes,
	pairToCommitsInCommonPerDayBucket map[int]map[string]commits.CommitHashes,
	componentOrFileToAllCommits map[string]commits.CommitHashes,
	componentOrFileToAllCommitsPerDayBucket map[int]map[string]commits.CommitHashes,
) []*core.Row {
	var rows []*core.Row

	for _, component1 := range componentsOrFiles {
		for _, component2 := range componentsOrFiles {
			if component1 == component2 {
				continue
			}

			combined := []string{component1, component2}
			slices.Sort(combined)
			key := strings.Join(combined, ":")

			pairsPerDayBucketMapped := lo.MapValues(pairToCommitsInCommonPerDayBucket, func(sharedCommitCount map[string]commits.CommitHashes, _ int) commits.CommitHashes {
				if _, hasKey := sharedCommitCount[key]; !hasKey {
					return sharedCommitCount[key]
				}
				return commits.CommitHashes{}
			})

			row1 := toRow(component1, component2, pairToCommitsInCommon[key], pairsPerDayBucketMapped, componentOrFileToAllCommits, componentOrFileToAllCommitsPerDayBucket)

			rows = append(rows, row1)
		}
	}
	return rows
}

func toRow(
	component1, component2 string,
	commitsInCommon commits.CommitHashes,
	commitsInCommonPerDayBucket map[int]commits.CommitHashes,
	componentOrFileToAllCommits map[string]commits.CommitHashes,
	componentOrFileToAllCommitsPerDayBucket map[int]map[string]commits.CommitHashes,
) *core.Row {
	row1 := &core.Row{
		Data: map[string]interface{}{
			Pair1: component1,
			Pair2: component2,
		},
	}
	row1.Data[SharedCommitCount] = len(commitsInCommon)
	row1.Data[PercentageOfAllCommitsPair1] = float64(len(commitsInCommon)) / float64(len(componentOrFileToAllCommits[component1])) * 100.0
	row1.Data[PercentageOfAllCommitsPair2] = float64(len(commitsInCommon)) / float64(len(componentOrFileToAllCommits[component2])) * 100.0

	for days, commitsInCommon := range commitsInCommonPerDayBucket {
		row1.Data[toDayStat(SharedCommitCount, days)] = len(commitsInCommon)
		row1.Data[toDayStat(PercentageOfAllCommitsPair1, days)] = float64(len(commitsInCommon)) / float64(len(componentOrFileToAllCommitsPerDayBucket[days][component1])) * 100.0
		row1.Data[toDayStat(PercentageOfAllCommitsPair2, days)] = float64(len(commitsInCommon)) / float64(len(componentOrFileToAllCommitsPerDayBucket[days][component2])) * 100.0
	}

	return row1
}
