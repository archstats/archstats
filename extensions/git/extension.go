package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
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
		DayBuckets:         []int{30, 90, 180},
		GenerateCommitView: true,
		GitAfter:           "",
		GitSince:           "",
		BasedOn:            time.Now(),
	}
}

type extension struct {
	// The number of days to bucket stats into. For example, if this is set to [7, 30, 90], then the stats will be bucketed into
	// 7 days, 30 days, and 90 days. This is useful for seeing code churn over time.
	DayBuckets []int
	// If true, a view will be generated that shows all commits
	GenerateCommitView bool
	// Passed to `git log --after`
	GitAfter string
	// Passed to `git log --since`
	GitSince string
	// What is the base date for the stats? If this is set, the aggregated time stats will be relative to this date.
	BasedOn time.Time

	// Represents an individual change in a commit. A commit can have multiple parts if it changes multiple files.
	commitParts []*partOfCommit

	commitPartsByFile      map[string][]*partOfCommit
	commitPartsByComponent map[string][]*partOfCommit
	commitPartsByCommit    map[string][]*partOfCommit
	commitPartsByAuthor    map[string][]*partOfCommit
	commitPartsByDayBucket map[int][]*partOfCommit
}

func (e *extension) Init(settings core.Analyzer) error {
	settings.RegisterResultsEditor(e)
	settings.RegisterView(&core.ViewFactory{
		Name:           "git_authors",
		CreateViewFunc: e.authorViewFactory,
	})
	settings.RegisterView(&core.ViewFactory{
		Name:           "git_component_logical_coupling",
		CreateViewFunc: e.componentCouplingViewFactory,
	})
	settings.RegisterView(&core.ViewFactory{
		Name:           "git_file_logical_coupling",
		CreateViewFunc: e.fileCouplingViewFactory,
	})
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

	e.commitParts = lo.FlatMap(rawCommits, func(commit *rawCommit, index int) []*partOfCommit {
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

	e.commitPartsByFile = splitByFile(e.commitParts)
	e.commitPartsByComponent = splitByComponent(e.commitParts)

	e.commitPartsByDayBucket = splitCommitsIntoBucketsOfDays(e.BasedOn, e.commitParts, e.DayBuckets)

	setStatsTotal(e.BasedOn, results.StatsByFile, e.commitPartsByFile)
	setStatsTotal(e.BasedOn, results.StatsByComponent, e.commitPartsByComponent)

	for days, commits := range e.commitPartsByDayBucket {

		byComponent := splitByComponent(commits)
		byFile := splitByFile(commits)

		setStatsLastXDays(e.BasedOn, days, results.StatsByFile, byFile)
		setStatsLastXDays(e.BasedOn, days, results.StatsByComponent, byComponent)
	}
}

type partOfCommit struct {
	component   string
	commit      string
	time        time.Time
	file        string
	author      string
	authorEmail string
	message     string
	additions   int
	deletions   int
}

func setComponent(results *core.Results, commitParts []*partOfCommit) {
	for _, part := range commitParts {
		part.component = results.FileToComponent[part.file]
	}
}

func gitCommitToPartOfCommit(commit *rawCommit) []*partOfCommit {
	return lo.Map(commit.Files, func(file *rawPartOfCommit, _ int) *partOfCommit {
		return &partOfCommit{
			component:   "",
			commit:      commit.Hash,
			time:        commit.Time,
			file:        file.Path,
			author:      commit.AuthorName,
			authorEmail: commit.AuthorEmail,
			message:     commit.Message,
			additions:   file.Additions,
			deletions:   file.Deletions,
		}
	})
}

func setStatsTotal(basedOn time.Time, statGroup file.StatsGroup, group map[string][]*partOfCommit) {
	for filePath, _ := range statGroup {
		commitParts := group[filePath]

		stats := getCommitStats(basedOn, commitParts)
		statGroup.SetStat(filePath, AdditionCount, stats.additionCount)
		statGroup.SetStat(filePath, DeletionCount, stats.deletionCount)
		statGroup.SetStat(filePath, CommitCount, stats.commitCount)
		statGroup.SetStat(filePath, AuthorCount, stats.uniqueAuthorCount)
		statGroup.SetStat(filePath, AgeInDays, stats.oldestCommitAgeInDays)
	}
}

func setStatsLastXDays(basedOn time.Time, days int, statGroup file.StatsGroup, group map[string][]*partOfCommit) {
	for filePath, _ := range group {
		commitParts := group[filePath]

		stats := getCommitStats(basedOn, commitParts)
		statGroup.SetStat(filePath, toDayStat(AdditionCount, days), stats.additionCount)
		statGroup.SetStat(filePath, toDayStat(DeletionCount, days), stats.deletionCount)
		statGroup.SetStat(filePath, toDayStat(CommitCount, days), stats.commitCount)
		statGroup.SetStat(filePath, toDayStat(AuthorCount, days), stats.uniqueAuthorCount)
	}
}
