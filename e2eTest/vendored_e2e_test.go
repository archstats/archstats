package e2eTest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Someone else's code carried in the repository is counted as files, and is
// no part of the architecture: moment.js's locale folder was one of
// nopCommerce's components.
func Test_Vendored_FilesAreCountedButFormNoComponent(t *testing.T) {
	var files []fileRow
	runView(t, "vendored", "files", "name,component,modularity__types__total", &files)
	byName := map[string]fileRow{}
	for _, f := range files {
		byName[f.Name] = f
	}
	vendored, found := byName["wwwroot/lib_npm/moment/locale/de.js"]
	require.Truef(t, found, "the vendored file is still a file; got %v", keysOf(byName))
	assert.Empty(t, vendored.Component)
	assert.Equal(t, "app", byName["app/main.js"].Component)

	var components []struct {
		Name string `csv:"NAME"`
	}
	runView(t, "vendored", "components", "name", &components)
	var names []string
	for _, c := range components {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"app"}, names)
}
