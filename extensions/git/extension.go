package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/rs/zerolog/log"
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
	Repository                 = "git__repository"

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

	rootPath string

	repositories []string
}

func (e *extension) AnalyzeFile(fileE file.File) *file.Results {

	path := fileE.Path()
	repo := getRepoFromFile(e.repositories, path)
	commitsByFile := e.splittedCommits.SplitByFile()
	commitsForFile := commitsByFile[path]
	splittedByDay := commits.Split(e.BasedOn, e.DayBuckets, commitsForFile).DayBuckets()

	var recordsToReturn []*stats.Record
	commitStats := commits.GetStats(e.BasedOn, commitsForFile)

	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: Repository, Value: repo})
	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: AgeInDays, Value: commitStats.OldestCommitAgeInDays})
	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toTotalStat(AdditionCount), Value: commitStats})
	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toTotalStat(DeletionCount), Value: commitStats})
	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toTotalStat(CommitCount), Value: commitStats})
	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toTotalStat(AuthorCount), Value: commitStats})
	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toTotalStat(UniqueFileChangeCount), Value: commitStats})

	for days, split := range splittedByDay {
		commitsForThisFile := split.CommitParts()
		bucketStats := commits.GetStats(e.BasedOn, commitsForThisFile)
		recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toDayStat(AdditionCount, days), Value: bucketStats})
		recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toDayStat(DeletionCount, days), Value: bucketStats})
		recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toDayStat(CommitCount, days), Value: bucketStats})
		recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toDayStat(AuthorCount, days), Value: bucketStats})
		recordsToReturn = append(recordsToReturn, &stats.Record{StatType: toDayStat(UniqueFileChangeCount, days), Value: bucketStats})

	}

	return &file.Results{
		Stats: recordsToReturn,
	}
}

func UniqueAuthors(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	authors := make(map[string]bool)
	for _, commit := range commitSlice {
		for _, author := range commit.UniqueAuthors {
			authors[author] = true
		}
	}
	return len(authors)
}

func UniqueCommits(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	commits := make(map[string]bool)
	for _, commit := range commitSlice {
		for _, commitHash := range commit.UniqueCommits {
			commits[commitHash] = true
		}
	}
	return len(commits)
}

func UniqueFiles(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	files := make(map[string]bool)
	for _, commit := range commitSlice {
		for _, file := range commit.FileChanges {
			files[file] = true
		}
	}
	return len(files)
}
func TotalAdditions(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	totalAdditions := 0
	for _, commit := range commitSlice {
		totalAdditions += commit.AdditionCount
	}
	return totalAdditions
}
func TotalDeletions(thingsToMerge []interface{}) interface{} {
	commitSlice := lo.Map(thingsToMerge, func(thing interface{}, _ int) *commits.CommitStats {
		return thing.(*commits.CommitStats)
	})
	totalDeletions := 0
	for _, commit := range commitSlice {
		totalDeletions += commit.DeletionCount
	}
	return totalDeletions
}

func (e *extension) Init(settings core.Analyzer) error {
	settings.RegisterResultsEditor(e)

	settings.RegisterStatAccumulator(Repository, stats.MostCommonStatMerger)
	settings.RegisterStatAccumulator(AgeInDays, stats.MostCommonStatMerger)
	settings.RegisterStatAccumulator(toTotalStat(AuthorCount), UniqueAuthors)
	settings.RegisterStatAccumulator(toTotalStat(CommitCount), UniqueCommits)
	settings.RegisterStatAccumulator(toTotalStat(UniqueFileChangeCount), UniqueFiles)
	settings.RegisterStatAccumulator(toTotalStat(AdditionCount), TotalAdditions)
	settings.RegisterStatAccumulator(toTotalStat(DeletionCount), TotalDeletions)

	for _, bucket := range e.DayBuckets {
		settings.RegisterStatAccumulator(toDayStat(AuthorCount, bucket), UniqueAuthors)
		settings.RegisterStatAccumulator(toDayStat(CommitCount, bucket), UniqueCommits)
		settings.RegisterStatAccumulator(toDayStat(UniqueFileChangeCount, bucket), UniqueFiles)
		settings.RegisterStatAccumulator(toDayStat(AdditionCount, bucket), TotalAdditions)
		settings.RegisterStatAccumulator(toDayStat(DeletionCount, bucket), TotalDeletions)
	}

	settings.RegisterFileAnalyzer(e)
	settings.RegisterView(&core.ViewFactory{
		Name:           "git_authors",
		CreateViewFunc: e.authorViewFactory,
	})

	settings.RegisterView(&core.ViewFactory{
		Name:           "git_repos",
		CreateViewFunc: e.repoViewFactory,
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

	gitRepos, err := findGitRepos(settings.RootPath())
	if err != nil {
		log.Err(err).Msg("Error finding git repos")
	}
	e.repositories = gitRepos
	e.rootPath = settings.RootPath()
	rawCommits, err := getGitCommitsFromAllReposConcurrently(e.rootPath, gitRepos)
	if err != nil {
		return err
	}
	e.commitParts = lo.FlatMap(rawCommits, func(commit *rawCommit, index int) []*commits.PartOfCommit {
		return gitCommitToPartOfCommit(settings.RootPath(), commit)
	})
	e.splittedCommits = commits.Split(e.BasedOn, e.DayBuckets, e.commitParts)

	return nil
}

// TODO add definitions after API is stable
func (e *extension) definitions() []*definitions.Definition {
	return []*definitions.Definition{}
}
func (e *extension) EditResults(results *core.Results) {
	setComponent(results, e.commitParts)

}
func setComponent(results *core.Results, commitParts []*commits.PartOfCommit) {
	for _, part := range commitParts {
		part.Component = results.FileToComponent[part.File]
	}
}
func gitCommitToPartOfCommit(rootPath string, rawCommit *rawCommit) []*commits.PartOfCommit {
	return lo.Map(rawCommit.Files, func(file *rawPartOfCommit, _ int) *commits.PartOfCommit {

		rawRepoName := rawCommit.Repo
		pathToRepo := trimRepoPath(rootPath, rawRepoName)

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

func trimRepoPath(rootPath string, rawRepoName string) string {
	pathToRepo := strings.TrimPrefix(rawRepoName, rootPath)
	if strings.HasPrefix(pathToRepo, "/") {
		pathToRepo = pathToRepo[1:]
	}
	return pathToRepo
}

func getDir(path string) string {
	if strings.Contains(path, "/") {
		return path[:strings.LastIndex(path, "/")]
	}
	return ""
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
