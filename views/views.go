package views

import (
	"archstats/snippets"
	"fmt"
)

// GetRowsFromResults returns the list of Rows based on the input command from the CLI
func GetRowsFromResults(command string, results *snippets.Results) (*View, error) {
	views := map[string]ViewFunction{
		"components":            ComponentView,
		"component-connections": ComponentConnectionsView,
		"files":                 FileView,
		"directories":           DirectoryView,
		"directories-recursive": DirectoryRecursiveView,
		"snippets":              SnippetsView,
	}

	if view, isAnAvailableView := views[command]; isAnAvailableView {
		return view(results), nil
	} else {
		return nil, fmt.Errorf("%s is not a recognized view", command)
	}
}

type ViewFunction func(results *snippets.Results) *View

type View struct {
	OrderedColumns []string
	Rows           []*Row
}
type Row struct {
	Data map[string]interface{}
}
