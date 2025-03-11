package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/extensions/util"
	"strings"
)

const (
	UnknownRepository = ""
)

func (e *extension) repoViewFactory(results *core.Results) *core.View {
	repositoryToStatRecords := make(map[string][]*stats.Record)
	for _, repository := range e.repositories {
		repositoryToStatRecords[repository] = make([]*stats.Record, 0)
	}
	repositoryToStatRecords[UnknownRepository] = make([]*stats.Record, 0)

	for file, records := range results.StatRecordsByFile {
		repoName := getRepoFromFile(e.repositories, file)
		repositoryToStatRecords[repoName] = append(repositoryToStatRecords[repoName], records...)
	}
	repositoryStats := results.CalculateAccumulatedStatRecords(repositoryToStatRecords)
	uniqueStats := util.GetDistinctColumnsFrom(repositoryStats)

	return util.GenericView(uniqueStats, repositoryStats)
}

func getRepoFromFile(availableRepositories []string, fileName string) string {
	for _, repository := range availableRepositories {
		if strings.HasPrefix(fileName, repository) {
			return repository
		}
	}
	return UnknownRepository
}
