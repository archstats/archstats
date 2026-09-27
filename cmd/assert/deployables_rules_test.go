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

// The shipped deployable rule templates run against a real scan: a template
// with a typo or a column that does not exist would otherwise reach whoever
// copies it first. The workspace breaks every one of them on purpose.
func TestDeployableRuleTemplatesRun(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	dir, err := os.MkdirTemp(wd, "archstats-deployable-rules-*")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	write := func(p, content string) {
		full := filepath.Join(dir, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	write("api/Dockerfile", "FROM eclipse-temurin:11\nCOPY . /app\n")
	write("api/App.java", "package api; class App {}")
	write("web/Dockerfile", "FROM node\nCOPY . /app\n")
	write("web/index.js", "")
	write("docker-compose.yml", "services:\n  api:\n    build: ./api\n    environment:\n      - DB=jdbc:postgresql://db/shop\n  web:\n    build: ./web\n    environment:\n      - DB=jdbc:postgresql://db/shop\n")
	write(".github/workflows/ci.yml", "on: push\njobs:\n  b:\n    uses: acme/templates/.github/workflows/build.yml@main\n  d:\n    steps:\n      - run: docker build -t acme/api ./api\n")

	rules, err := filepath.Abs(filepath.Join("..", "..", "docs", "rules", "deployables.yml"))
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
		"Every deployable is built by a pipeline here",
		"No pipeline hands its work to a template that follows a branch",
		"No two deployables share a database",
		"No Java runtime below 17",
		"Every base image names a version",
		"Every deployable runs in production",
	} {
		assert.Contains(t, text, "RULE VIOLATED: "+broken)
	}
}
