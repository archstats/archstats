package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/extensions/util"
	"strings"
	"time"
)

const (
	UnknownRepository = ""
)

func (e *extension) repoViewFactory(results *core.Results) *core.View {
	repositoryToStatRecords := make(map[string][]*stats.Record)
	for _, repository := range e.repositories {
		if repository == "" {
			repository = RootRepository
		}
		repositoryToStatRecords[repository] = make([]*stats.Record, 0)
	}
	repositoryToStatRecords[UnknownRepository] = make([]*stats.Record, 0)

	for file, records := range results.StatRecordsByFile {
		repoName := getRepoFromFile(e.repositories, file)
		repositoryToStatRecords[repoName] = append(repositoryToStatRecords[repoName], records...)
	}
	repositoryStats := results.CalculateAccumulatedStatRecords(repositoryToStatRecords)
	uniqueStats := util.GetDistinctColumnsFrom(repositoryStats)

	view := util.GenericView(uniqueStats, repositoryStats)
	// Whether the history behind every git number is complete. 1 for a
	// shallow clone, whose history stops at the depth it was cloned with.
	view.Columns = append(view.Columns, core.IntColumn(ShallowClone),
		core.StringColumn(HeadCommit), core.StringColumn(HeadBranch), core.StringColumn(HeadTime),
		core.IntColumn(DirtyFiles), core.IntColumn(SweepingCommits))
	for _, row := range view.Rows {
		name, _ := row.Data["name"].(string)
		if name == RootRepository {
			name = ""
		}
		shallow := 0
		if e.shallow[name] {
			shallow = 1
		}
		row.Data[ShallowClone] = shallow
		// Which code this repository was when scanned; empty for the
		// "unknown" bucket of files outside every repository.
		if h, ok := e.heads[name]; ok {
			row.Data[HeadCommit] = h.Commit
			row.Data[HeadBranch] = h.Branch
			if !h.Time.IsZero() {
				row.Data[HeadTime] = h.Time.UTC().Format(time.RFC3339)
			}
			row.Data[DirtyFiles] = h.Dirty
		}
		row.Data[SweepingCommits] = e.sweepingByRepo[name]
	}
	return view
}

const (
	ShallowClone    = "git__shallow_clone"
	HeadCommit      = "git__head_commit"
	HeadBranch      = "git__branch"
	HeadTime        = "git__head_time"
	DirtyFiles      = "git__dirty_files"
	SweepingCommits = "git__sweeping_commits"
)

// RootRepository names a repository at the scan root, which has no path of
// its own. It used to be "", the same as UnknownRepository, so every file of
// a single-repository scan was reported under "Unknown".
const RootRepository = "."

// getRepoFromFile is the innermost repository holding a file. A plain prefix
// test put "api-gateway/x.go" in the repository "api", and returned whichever
// repository happened to be listed first.
func getRepoFromFile(availableRepositories []string, fileName string) string {
	fileName = strings.TrimPrefix(fileName, "./")
	best, found := "", false
	for _, repository := range availableRepositories {
		if repository != "" && !strings.HasPrefix(fileName, repository+"/") {
			continue
		}
		if !found || len(repository) > len(best) {
			best, found = repository, true
		}
	}
	if !found {
		return UnknownRepository
	}
	if best == "" {
		return RootRepository
	}
	return best
}
