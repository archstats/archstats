package common

import (
	"path"
	"strings"

	"github.com/archstats/archstats/core/file"
)

// KeepReactOnlyWhereUsed drops the React readings from a file that does not
// use React.
//
// "A React component" was any function whose name starts with a capital
// letter, which is also every constructor in plain JavaScript: jQuery and
// moment.js alone made a Java codebase report 263 React components, and an
// ASP.NET one 1,109. A file counts as React when it is JSX (.jsx/.tsx) or
// imports react, react-dom or preact.
func KeepReactOnlyWhereUsed(filePath string, res *file.Results) {
	if usesReact(filePath, res) {
		return
	}
	stats := res.Stats[:0]
	for _, r := range res.Stats {
		if !strings.Contains(r.StatType, "__react__") {
			stats = append(stats, r)
		}
	}
	res.Stats = stats
	snippets := res.Snippets[:0]
	for _, s := range res.Snippets {
		if !strings.Contains(s.Type, "__react__") {
			snippets = append(snippets, s)
		}
	}
	res.Snippets = snippets
}

func usesReact(filePath string, res *file.Results) bool {
	switch strings.ToLower(path.Ext(filePath)) {
	case ".jsx", ".tsx":
		return true
	}
	for _, s := range res.Snippets {
		if s.Type != "modularity__import__raw" {
			continue
		}
		spec := strings.Trim(s.Value, "\"'`")
		for _, pkg := range []string{"react", "react-dom", "preact"} {
			if spec == pkg || strings.HasPrefix(spec, pkg+"/") {
				return true
			}
		}
	}
	return false
}
