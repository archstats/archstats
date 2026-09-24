package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/regex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A --snippet pattern records a snippet of the type its group names.
func TestSnippetExtensionRecordsNamedGroups(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("TODO: one\nfine\nTODO: two\n"), 0o644))
	ext, err := SnippetExtension([]string{`TODO: (?P<todo>\w+)`})
	require.NoError(t, err)
	results, err := core.New(&core.Config{RootPath: root, Extensions: []core.Extension{ext}}).Analyze()
	require.NoError(t, err)
	var values []string
	for _, s := range results.Snippets {
		if s.Type == "todo" {
			values = append(values, s.Value)
		}
	}
	assert.ElementsMatch(t, []string{"one", "two"}, values)
}

func TestSnippetExtensionRejectsBadPatterns(t *testing.T) {
	_, err := SnippetExtension([]string{`(unclosed`})
	assert.Error(t, err)
	_, err = SnippetExtension([]string{`TODO: \w+`})
	assert.ErrorContains(t, err, "named group")
	ext, err := SnippetExtension([]string{`(?P<fn>func)`})
	require.NoError(t, err)
	assert.IsType(t, &regex.Extension{}, ext)
}
