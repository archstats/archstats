package git

import (
	"embed"
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/core/walker"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/archstats/archstats/extensions/util"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"slices"
	"strings"
	"time"
)

//go:embed definitions/**
var gitDefs embed.FS

const (
	AuthorCount = "git__authors"
	AgeInDays   = "git__age_in_days"
	// LastChangeAgeInDays is days from the file's last change to the anchor
	// (the newest commit scanned): how long code has sat untouched, which
	// git__age_in_days -- days since it first appeared -- cannot say.
	LastChangeAgeInDays        = "git__last_change_age_in_days"
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
		MaxChangesPerCommit:                  100,
		BasedOn:                              time.Now(),
		BasedOnMode:                          "head",
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

	// Filter out commits that modify more than this number of files
	MaxChangesPerCommit int

	// What is the base date for the stats? If this is set, the aggregated time stats will be relative to this date.
	BasedOn time.Time
	// BasedOnMode is "head": the windows count back from the newest commit
	// scanned, so scanning the same commit a day apart reads the same and a
	// dormant repository's last 30 days are its last 30 days of work; or
	// "now": from the moment of the scan, as before revision 2.
	BasedOnMode string

	// Represents an individual change in a commit. A commit can have multiple parts if it changes multiple files.
	commitParts []*commits.PartOfCommit
	// viewParts is every row, pure moves included, for the commit table.
	viewParts []*commits.PartOfCommit
	// The subset of commitParts co-change is measured on: sweeping commits
	// left out.
	couplingParts   []*commits.PartOfCommit
	couplingCommits *commits.Splitted

	splittedCommits *commits.Splitted

	rootPath string

	repositories []string
	// What each repository's working tree was when it was scanned.
	heads map[string]headInfo
	// Commits left out of co-change for touching too many files, in total
	// and per repository.
	sweepingTotal int
	// Rows of history for files the workspace's ignore patterns exclude:
	// dropped, and counted so the snapshot can say so.
	ignoredRows    int
	sweepingByRepo map[string]int
	// Repositories whose history was cut short by a shallow clone.
	shallow map[string]bool
}

func (e *extension) AnalyzeFile(fileE file.File) *file.Results {

	path := fileE.Path()
	repo := getRepoFromFile(e.repositories, path)
	commitsByFile := e.splittedCommits.SplitByFile()
	commitsForFile := commitsByFile[path]

	// Skip files with no commit history
	if len(commitsForFile) == 0 {
		return nil
	}

	splittedByDay := commits.Split(e.BasedOn, e.DayBuckets, commitsForFile).DayBuckets()

	var recordsToReturn []*stats.Record
	commitStats := commits.GetStats(e.BasedOn, commitsForFile)

	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: Repository, Value: repo})
	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: AgeInDays, Value: commitStats.OldestCommitAgeInDays})
	recordsToReturn = append(recordsToReturn, &stats.Record{StatType: LastChangeAgeInDays, Value: commitStats.NewestCommitAgeInDays})
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

func (e *extension) Init(settings core.Analyzer) error {
	settings.RegisterResultsEditor(e)

	loadedDefs, err := definitions.LoadYamlFiles(gitDefs)
	if err != nil {
		return err
	}
	for _, def := range loadedDefs {
		settings.AddDefinition(def)
	}

	settings.RegisterStatAccumulator(Repository, stats.MostCommonStatMerger)
	// A group is as old as its oldest file. The most common file age said
	// nothing about the group at all.
	settings.RegisterStatAccumulator(AgeInDays, maxIntMerger)
	// A group last changed when its most recently changed file did.
	settings.RegisterStatAccumulator(LastChangeAgeInDays, minIntMerger)
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

		settings.AddDefinition(&definitions.Definition{
			Id:               toDayStat(AdditionCount, bucket),
			Name:             fmt.Sprintf("Addition Count (Last %d Days)", bucket),
			ShortDescription: fmt.Sprintf("Number of lines of code added to this file or component in the last %d days.", bucket),
			LongDescription:  fmt.Sprintf("Counts how many new lines of code were added in git commits over the last %d days. Helps identify recently active or growing files.", bucket),
			Category:         "Git History & Churn",
		})
		settings.AddDefinition(&definitions.Definition{
			Id:               toDayStat(DeletionCount, bucket),
			Name:             fmt.Sprintf("Deletion Count (Last %d Days)", bucket),
			ShortDescription: fmt.Sprintf("Number of lines of code deleted from this file or component in the last %d days.", bucket),
			LongDescription:  fmt.Sprintf("Counts how many lines of code were deleted in git commits over the last %d days. Helps identify recently refactored or cleaned-up files.", bucket),
			Category:         "Git History & Churn",
		})
		settings.AddDefinition(&definitions.Definition{
			Id:               toDayStat(CommitCount, bucket),
			Name:             fmt.Sprintf("Commit Count (Last %d Days)", bucket),
			ShortDescription: fmt.Sprintf("Number of unique git commits modifying this file or component in the last %d days.", bucket),
			LongDescription:  fmt.Sprintf("Counts how many times this file or component was modified in a git commit over the last %d days. A high count suggests a recent hot spot of activity.", bucket),
			Category:         "Git History & Churn",
		})
		settings.AddDefinition(&definitions.Definition{
			Id:               toDayStat(AuthorCount, bucket),
			Name:             fmt.Sprintf("Author Count (Last %d Days)", bucket),
			ShortDescription: fmt.Sprintf("Number of unique authors who modified this file or component in the last %d days.", bucket),
			LongDescription:  fmt.Sprintf("Counts how many different developers made changes to this file or component in the last %d days. High author count can indicate a risk of coordination issues.", bucket),
			Category:         "Git History & Churn",
		})
		settings.AddDefinition(&definitions.Definition{
			Id:               toDayStat(UniqueFileChangeCount, bucket),
			Name:             fmt.Sprintf("Unique File Changes (Last %d Days)", bucket),
			ShortDescription: fmt.Sprintf("Number of unique files changed in commits that touched this file or component in the last %d days.", bucket),
			LongDescription:  fmt.Sprintf("Measures co-changes over the last %d days: how many other files were modified in the same commits as this one. Helps find implicit logical coupling.", bucket),
			Category:         "Git History & Churn",
		})
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

	scan := findGitRepos(settings.RootPath())
	for _, dir := range scan.unreadable {
		log.Warn().Msgf("Could not read %s while looking for git repositories, so any repository under it is missing from this scan.", dir)
	}
	e.repositories = scan.repos
	warnAboutLazyRepos(settings.RootPath(), scan.repos)
	e.shallow = shallowRepos(settings.RootPath(), scan.repos)
	e.heads = readHeads(settings.RootPath(), scan.repos)
	e.rootPath = settings.RootPath()
	rawCommits, err := e.getGitCommitsFromAllReposConcurrently(e.rootPath, scan.repos)
	log.Info().Msgf("Found %d commits total across %d repositories", len(rawCommits), len(scan.repos))
	if err != nil {
		return err
	}
	canonicalizeAuthors(rawCommits)

	if e.BasedOnMode != "now" {
		newest := time.Time{}
		for _, h := range e.heads {
			if h.Time.After(newest) {
				newest = h.Time
			}
		}
		for _, c := range rawCommits {
			if c.Time.After(newest) {
				newest = c.Time
			}
		}
		if !newest.IsZero() {
			e.BasedOn = newest
		}
	}

	// Every commit counts for what happened to a file: its age, its churn,
	// who touched it. A copyright sweep over 3,000 files is still a change to
	// each of them; dropping it left files that were fifteen years old
	// reading as new and never changed. Sweeps are noise only for the
	// question "which files change together", so only co-change filters them.
	var couplingCommits []*rawCommit
	var excludedCount int
	for _, commit := range rawCommits {
		if e.MaxChangesPerCommit > 0 && len(commit.Files) > e.MaxChangesPerCommit {
			log.Debug().Msgf("Leaving sweeping commit %s (%d files changed) out of co-change: %s", commit.Hash, len(commit.Files), commit.Message)
			excludedCount++
			if e.sweepingByRepo == nil {
				e.sweepingByRepo = map[string]int{}
			}
			e.sweepingByRepo[repoName(e.rootPath, commit.Repo)]++
			continue
		}
		couplingCommits = append(couplingCommits, commit)
	}
	e.sweepingTotal = excludedCount
	if excludedCount > 0 {
		log.Info().Msgf("Left %d sweeping commits (modifying > %d files) out of co-change, of %d total commits", excludedCount, e.MaxChangesPerCommit, len(rawCommits))
	}

	// A file the workspace excludes is not part of the code read, so its
	// history is not either: its rows leave every table.
	excluded := walker.Matcher(settings.IgnorePatterns())
	partsByCommit := make(map[*rawCommit][]*commits.PartOfCommit, len(rawCommits))
	for _, commit := range rawCommits {
		parts := gitCommitToPartOfCommit(settings.RootPath(), commit)
		if len(settings.IgnorePatterns()) > 0 {
			kept := parts[:0]
			for _, p := range parts {
				if excluded(p.File) {
					e.ignoredRows++
					continue
				}
				kept = append(kept, p)
			}
			parts = kept
		}
		partsByCommit[commit] = parts
		// Every row is evidence in the commit table; a move that changed no
		// lines is not a change to count or to couple on.
		e.viewParts = append(e.viewParts, parts...)
		e.commitParts = append(e.commitParts, countable(parts)...)
	}
	for _, commit := range couplingCommits {
		e.couplingParts = append(e.couplingParts, countable(partsByCommit[commit])...)
	}
	e.splitAll()

	return nil
}

// splitAll indexes both commit sets. They share their PartOfCommit values, so
// a component set on one is set on the other.
func (e *extension) splitAll() {
	e.splittedCommits = commits.Split(e.BasedOn, e.DayBuckets, e.commitParts)
	e.couplingCommits = commits.Split(e.BasedOn, e.DayBuckets, e.couplingParts)
}

// TODO add definitions after API is stable
func (e *extension) definitions() []*definitions.Definition {
	return []*definitions.Definition{}
}
func (e *extension) EditResults(results *core.Results) {
	e.recordSnapshotInfo(results.SetSnapshotInfo)
	setComponent(results, e.viewParts)
	// Re-split commits based on components
	// This is necessary because components aren't known on Init()
	e.splitAll()

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

		filePath := asWalked(trimLeadingSlash(pathToRepo + "/" + file.Path))
		return &commits.PartOfCommit{
			Component: "",
			Repo:      pathToRepo,
			Commit:    rawCommit.Hash,
			Time:      rawCommit.Time,
			File:      filePath,
			// From the path under the scan root, not under the repository:
			// for a repository in a subfolder the two differ.
			Directory:    getDir(strings.TrimPrefix(filePath, "./")),
			Author:       rawCommit.AuthorName,
			AuthorEmail:  rawCommit.AuthorEmail,
			Message:      rawCommit.Message,
			Additions:    file.Additions,
			Deletions:    file.Deletions,
			PathAtCommit: asWalked(trimLeadingSlash(pathToRepo + "/" + orPath(file.PathAtCommit, file.Path))),
			ChangeKind:   file.ChangeKind,
			PureRename:   file.ChangeKind == ChangeRename && file.Additions == 0 && file.Deletions == 0,
		}
	})
}

// asWalked names a file the way the walker does. The walker starts from "."
// and names a file at the scan root "./go.mod"; git names it "go.mod". Every
// root-level file -- 52 of gin's 129 -- was reported as having no history.
func asWalked(path string) string {
	if !strings.Contains(path, "/") {
		return "./" + path
	}
	return path
}

// repoName is a commit's repository as the repositories list names it: ""
// for a repository at the scan root, its relative path otherwise.
func repoName(rootPath, rawRepo string) string {
	name := trimLeadingSlash(trimRepoPath(rootPath, rawRepo))
	if name == "." {
		return ""
	}
	return name
}

func trimRepoPath(rootPath string, rawRepoName string) string {
	pathToRepo := strings.TrimPrefix(rawRepoName, rootPath)
	return trimLeadingSlash(pathToRepo)
}

func trimLeadingSlash(path string) string {
	if strings.HasPrefix(path, "/") {
		return path[1:]
	}
	return path
}

func getDir(path string) string {
	if strings.Contains(path, "/") {
		return trimLeadingSlash(path[:strings.LastIndex(path, "/")])
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
			// Filter out pairs that don't have shared commits
			if _, hasKey := pairToCommitsInCommon[key]; !hasKey {
				continue
			}

			// The two branches were the wrong way round: a bucket that HAD
			// shared commits for this pair returned an empty set, and one
			// that did not returned the missing value. Every
			// __LAST_30_DAYS / __LAST_90_DAYS / __LAST_180_DAYS shared-commit
			// column came out zero on every repository, however recent the
			// work -- 100,572 rows of Sylius, all zero.
			pairsPerDayBucketMapped := lo.MapValues(pairToCommitsInCommonPerDayBucket, func(sharedCommitCount map[string]commits.CommitHashes, _ int) commits.CommitHashes {
				if shared, hasKey := sharedCommitCount[key]; hasKey {
					return shared
				}
				return commits.CommitHashes{}
			})

			row1 := toRow(component1, component2, pairToCommitsInCommon[key], pairsPerDayBucketMapped, componentOrFileToAllCommits, componentOrFileToAllCommitsPerDayBucket)
			// Only add rows that have shared commits
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
	row1.Data[PercentageOfAllCommitsPair1] = util.FiniteOrZero(float64(len(commitsInCommon)) / float64(len(componentOrFileToAllCommits[component1])) * 100.0)
	row1.Data[PercentageOfAllCommitsPair2] = util.FiniteOrZero(float64(len(commitsInCommon)) / float64(len(componentOrFileToAllCommits[component2])) * 100.0)

	for days, commitsInCommon := range commitsInCommonPerDayBucket {
		row1.Data[toDayStat(SharedCommitCount, days)] = len(commitsInCommon)
		row1.Data[toDayStat(PercentageOfAllCommitsPair1, days)] = util.FiniteOrZero(float64(len(commitsInCommon)) / float64(len(componentOrFileToAllCommitsPerDayBucket[days][component1])) * 100.0)
		row1.Data[toDayStat(PercentageOfAllCommitsPair2, days)] = util.FiniteOrZero(float64(len(commitsInCommon)) / float64(len(componentOrFileToAllCommitsPerDayBucket[days][component2])) * 100.0)
	}

	return row1
}

func minIntMerger(values []interface{}) interface{} {
	best, found := 0, false
	for _, v := range values {
		n, ok := v.(int)
		if ok && (!found || n < best) {
			best, found = n, true
		}
	}
	if !found {
		return nil
	}
	return best
}

func maxIntMerger(values []interface{}) interface{} {
	best, found := 0, false
	for _, v := range values {
		if i, ok := v.(int); ok && (!found || i > best) {
			best, found = i, true
		}
	}
	if !found {
		return nil
	}
	return best
}

// countable leaves out pure moves.
func countable(parts []*commits.PartOfCommit) []*commits.PartOfCommit {
	out := make([]*commits.PartOfCommit, 0, len(parts))
	for _, p := range parts {
		if !p.PureRename {
			out = append(out, p)
		}
	}
	return out
}

func orPath(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
