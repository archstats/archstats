package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A commit's files must carry the names the walker gave them, or they join
// nothing. The walker names a root-level file "./go.mod"; git says "go.mod",
// and every file at the root of gin was reported as never committed.
func TestCommitPathsMatchTheWalker(t *testing.T) {
	c := &rawCommit{Repo: "/scan", Files: []*rawPartOfCommit{{Path: "go.mod"}, {Path: "binding/json.go"}}}
	parts := gitCommitToPartOfCommit("/scan", c)
	assert.Equal(t, "./go.mod", parts[0].File)
	assert.Equal(t, "", parts[0].Directory)
	assert.Equal(t, "binding/json.go", parts[1].File)
	assert.Equal(t, "binding", parts[1].Directory)

	// A repository in a subfolder of the scan root: its root files are not
	// at the scan root, and its directories are under the subfolder.
	nested := &rawCommit{Repo: "/scan/api", Files: []*rawPartOfCommit{{Path: "go.mod"}, {Path: "handler/h.go"}}}
	parts = gitCommitToPartOfCommit("/scan", nested)
	assert.Equal(t, "api/go.mod", parts[0].File)
	assert.Equal(t, "api", parts[0].Directory)
	assert.Equal(t, "api/handler/h.go", parts[1].File)
	assert.Equal(t, "api/handler", parts[1].Directory)
}

func TestAFileBelongsToTheInnermostRepositoryHoldingIt(t *testing.T) {
	repos := []string{"", "api", "api/plugins"}
	assert.Equal(t, RootRepository, getRepoFromFile(repos, "./go.mod"))
	assert.Equal(t, RootRepository, getRepoFromFile(repos, "api-gateway/main.go"))
	assert.Equal(t, "api", getRepoFromFile(repos, "api/main.go"))
	assert.Equal(t, "api/plugins", getRepoFromFile(repos, "api/plugins/p.go"))
	assert.Equal(t, UnknownRepository, getRepoFromFile([]string{"api"}, "web/index.ts"))
}
