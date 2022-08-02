package views

import (
	"fmt"
	"github.com/RyanSusana/archstats/snippets"
)

func GetAllViews(results *snippets.Results) map[string]*View {
	views := getViewFunctionMap()
	allViews := make(map[string]*View, len(views))

	for viewName, view := range views {
		allViews[viewName] = view(results)
	}
	return allViews
}

// GetView returns the list of Rows based on the input command from the CLI
func GetView(command string, results *snippets.Results) (*View, error) {
	views := getViewFunctionMap()
	if view, isAnAvailableView := views[command]; isAnAvailableView {
		return view(results), nil
	} else {
		return nil, fmt.Errorf("%s is not a recognized view", command)
	}
}

func getViewFunctionMap() map[string]ViewFunction {
	return map[string]ViewFunction{
		"summary":               SummaryView,
		"components":            ComponentView,
		"component-connections": ComponentConnectionsView,
		"files":                 FileView,
		"directories":           DirectoryView,
		"directories-recursive": DirectoryRecursiveView,
		//"snippets":              SnippetsView, TODO: this is a noisy, not insightful, view. But it's handy for something like `--raw-snippets`
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
