package views

import (
	"fmt"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/samber/lo"
)

const (
	SummaryViewName                          = "summary"
	ComponentViewName                        = "components"
	ComponentConnectionsViewName             = "component_connections"
	ComponentCyclesViewName                  = "all_component_cycles"
	LargestComponentCycleViewName            = "largest_component_cycle"
	StronglyConnectedComponentGroupsViewName = "strongly_connected_component_groups"
	FileViewName                             = "files"
	DirectoryViewName                        = "directories"
	DirectoryRecursiveViewName               = "directories_recursive"
	SnippetsViewName                         = "snippets"
)

// RenderView returns the list of Rows based on the input command from the CLI
func RenderView(command string, results *analysis.Results) (*View, error) {
	views := getViewFactories()
	if view, isAnAvailableView := views[command]; isAnAvailableView {
		v := view(results)
		v.Name = command
		return v, nil
	} else {
		return nil, fmt.Errorf("%s is not a recognized view", command)
	}
}
func GetQuickViews() []string {
	return lo.Without(GetAvailableViews(), []string{ComponentCyclesViewName, LargestComponentCycleViewName}...)
}
func GetAvailableViews() []string {
	views := getViewFactories()
	availableViews := make([]string, 0, len(views))
	for viewName := range views {
		availableViews = append(availableViews, viewName)
	}
	return availableViews
}
func getViewFactories() map[string]ViewFactory {
	return map[string]ViewFactory{
		SummaryViewName:                          SummaryView,
		ComponentViewName:                        ComponentView,
		ComponentConnectionsViewName:             ComponentConnectionsView,
		ComponentCyclesViewName:                  ComponentCyclesView,
		LargestComponentCycleViewName:            LargestComponentCycleView,
		StronglyConnectedComponentGroupsViewName: StronglyConnectedComponentGroupsView,
		FileViewName:                             FileView,
		DirectoryViewName:                        DirectoryView,
		DirectoryRecursiveViewName:               DirectoryRecursiveView,
		SnippetsViewName:                         SnippetsView,
	}
}

type ViewFactory func(results *analysis.Results) *View

type View struct {
	Name    string
	Columns []*Column
	Rows    []*Row
}
type Row struct {
	Data map[string]interface{}
}

const (
	Integer = iota
	Float
	String
	Date
)

type Column struct {
	Name string
	Type int
}

func StringColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: String,
	}
}
func IntColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Integer,
	}
}

func FloatColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Float,
	}
}
func DateColumn(name string) *Column {
	return &Column{
		Name: name,
		Type: Date,
	}
}
