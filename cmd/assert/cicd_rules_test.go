package assert_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/archstats/archstats/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shipped CI/CD rule templates run against a real scan: a template
// with a typo or a column that does not exist would otherwise reach whoever
// copies it first. The workspace breaks every one of them on purpose.
func TestCICDRuleTemplatesRun(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	dir, err := os.MkdirTemp(wd, "archstats-cicd-rules-*")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	write := func(p, content string) {
		full := filepath.Join(dir, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	write("api/Dockerfile", "FROM eclipse-temurin:11\nCOPY . /app\n")
	write("api/App.java", "package api; class App {}")
	write("web/Dockerfile", "FROM node\nUSER node\nCOPY src /app\n")
	write("web/src/index.js", "")
	write(".github/workflows/ci.yml", "on: push\njobs:\n  b:\n    uses: acme/templates/.github/workflows/build.yml@main\n  d:\n    runs-on: ubuntu-latest\n    environment: production\n    steps:\n      - run: curl -sL https://x.sh | sh\n")
	write(".gitlab-ci.yml", "deploy-prd:\n  script:\n    - rancher login $SERVER --token $T --context $RANCHER_PROJECT_PRD --skip-verify\n    - rancher kubectl apply -f k8s/\n")
	rules, err := filepath.Abs(filepath.Join("..", "..", "docs", "rules", "cicd.yml"))
	require.NoError(t, err)
	root, err := cmd.Cmd()
	require.NoError(t, err)
	out := new(bytes.Buffer)
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs([]string{"assert", "-r", rules, "-f", dir})
	err = root.Execute()
	text := out.String()
	assert.Error(t, err, "the workspace breaks rules on purpose")
	assert.NotContains(t, strings.ToLower(text), "no such column")
	assert.NotContains(t, strings.ToLower(text), "syntax error")
	for _, broken := range []string{
		"Every image is built by a pipeline here",
		"Every base image names a version",
		"No image runs as root",
		"No COPY of everything without a .dockerignore",
		"Every action, orb, pipe and component is pinned",
		"Every GitHub workflow sets its permissions",
		"No script pipes a download into a shell",
		"TLS verification stays on",
		"No deploy to production without a gate",
	} {
		assert.Contains(t, text, "RULE VIOLATED: "+broken)
	}
}
