package treesitter

import (
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/archstats/archstats/extensions/components/declbased"
	"github.com/archstats/archstats/extensions/regex"
	"github.com/archstats/archstats/extensions/treesitter/java"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const TestDataPath = "./temp_testdata"

func Test(t *testing.T) {
	createElepyRepo(t)

	path := "temp_testdata/elepy"
	regexJavaExt, err := regex.BuiltInRegexExtension("java")
	analysisTreesitter := core.New(&core.Config{
		RootPath: path,
		Extensions: []core.Extension{
			&java.Extension{},
			declbased.Extension(),
		},
	})
	analysisRegex := core.New(&core.Config{
		RootPath: path,
		Extensions: []core.Extension{
			regexJavaExt,
			declbased.Extension(),
		},
	})
	resultsTreesitter, err := analysisTreesitter.Analyze()

	resultsRegex, err := analysisRegex.Analyze()
	if err != nil {
		t.Error(err)
	}

	treesitterComponents, regexComponents := getComponents(resultsTreesitter), getComponents(resultsRegex)
	assert.ElementsMatch(t, treesitterComponents, regexComponents)

	for _, component := range treesitterComponents {
		treesitterFiles, regexFiles := resultsTreesitter.ComponentToFiles[component], resultsRegex.ComponentToFiles[component]
		assert.ElementsMatch(t, treesitterFiles, regexFiles)

		treesitterConnectionsTo, regexConnectionsTo := stringifyConnectionsMap(resultsTreesitter.ConnectionsFrom[component]), stringifyConnectionsMap(resultsRegex.ConnectionsFrom[component])
		assert.ElementsMatch(t, treesitterConnectionsTo, regexConnectionsTo)

	}

	fmt.Println("done")
}

func stringifyConnectionsMap(connectionsMap []*component.Connection) []string {
	return lo.Map(connectionsMap, func(connection *component.Connection, idx int) string {
		return connection.String()
	})
}
func getComponents(results *core.Results) []string {
	return lo.Keys(results.SnippetsByComponent)
}

func createElepyRepo(t *testing.T) {
	repo := "https://github.com/RyanSusana/elepy"
	commit := "472ccc0df35bc3dbfafab1e6d3a4caf309382f92"
	repoDirName := strings.ReplaceAll(fmt.Sprintf("%s/%s", TestDataPath, repo[strings.LastIndex(repo, "/"):]), ".git", "")
	os.Mkdir(TestDataPath, 0755)

	if _, err := os.Stat(repoDirName); os.IsNotExist(err) {
		err := exec.Command("git", "clone", repo, repoDirName).Run()
		if err != nil {
			assert.Fail(t, "Failed to clone repo: %s, %s", repo, err)
		}
	} else {
		log.Println("Repo already exists, skipping clone...")
	}

	err := exec.Command("git", "-C", repoDirName, "reset", "--hard", commit).Run()
	if err != nil {
		assert.Fail(t, "Failed to reset repo (%s) to commit '%s'", repo, commit)
	}
}
