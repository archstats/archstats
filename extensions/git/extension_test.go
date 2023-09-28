//go:build skip
// +build skip

package git

import (
	"fmt"
	"github.com/RyanSusana/archstats/core"
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
	analyzer := core.New("temp_testdata/ktor", []core.Extension{&Extension{}})
	results, err := core.Analyze(analyzer)
	if err != nil {
		return
	}

	results.GetAllViewFactories()
}

func createElepyRepo(t *testing.T) {
	repo := "https://github.com/ktorio/ktor.git"
	commit := "dc2259170362ad637055ff39668a7a3572c637e0"
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
