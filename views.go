package main

import (
	"archstats/snippets"
	"fmt"
	"golang.org/x/exp/slices"
	"math"
	"sort"
)

const (
	AfferentCouplings    = "afferent_couplings"
	EfferentCouplings    = "efferent_couplings"
	Instability          = "instability"
	Abstractness         = "abstractness"
	DistanceMainSequence = "distance_main_sequence"
)

// getRowsFromResults returns the list of rows based on the input command from the CLI
func getRowsFromResults(command string, results *snippets.Results) (*View, error) {
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
	rows           []*Row
}
type Row struct {
	Data map[string]interface{}
}

func ComponentConnectionsView(results *snippets.Results) *View {
	connections := make([]*Row, 0, len(results.Connections))
	grouped := snippets.GroupConnectionsBy(results.Connections, func(connection *snippets.ComponentConnection) string {
		return connection.From + " -> " + connection.To
	})

	for connectionName, groupedConnections := range grouped {
		connections = append(connections, &Row{
			Data: map[string]interface{}{
				"name":  connectionName,
				"from":  groupedConnections[0].From,
				"to":    groupedConnections[0].To,
				"count": len(groupedConnections),
			},
		})
	}
	return &View{
		OrderedColumns: []string{"from", "to", "count"},
		rows:           connections,
	}
}

func DirectoryView(results *snippets.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.SnippetsByDirectory)
}
func ComponentView(results *snippets.Results) *View {
	view := GenericView(getDistinctColumnsFromResults(results), results.SnippetsByComponent)

	for _, row := range view.rows {
		component := row.Data["name"].(string)
		afferentCouplings, efferentCouplings := len(results.ConnectionsTo[component]), len(results.ConnectionsFrom[component])
		abstractness := row.Data["abstractness"].(float64)
		instability := math.Max(0, math.Min(1, float64(efferentCouplings)/float64(afferentCouplings+efferentCouplings)))
		distanceMainSequence := abstractness + instability - 1

		row.Data[AfferentCouplings] = afferentCouplings
		row.Data[EfferentCouplings] = efferentCouplings
		row.Data[Instability] = instability
		row.Data[DistanceMainSequence] = distanceMainSequence
	}
	view.OrderedColumns = append(view.OrderedColumns, AfferentCouplings, EfferentCouplings, Instability)

	return view
}
func FileView(results *snippets.Results) *View {
	return GenericView(getDistinctColumnsFromResults(results), results.SnippetsByFile)
}

func SnippetsView(results *snippets.Results) *View {
	toReturn := make([]*Row, 0, len(results.Snippets))
	for _, snippet := range results.Snippets {
		toReturn = append(toReturn, &Row{
			Data: map[string]interface{}{
				"file":      snippet.File,
				"directory": snippet.Directory,
				"component": snippet.Component,
				"type":      snippet.Type,
				"begin":     snippet.Begin,
				"end":       snippet.End,
				"value":     snippet.Value,
			},
		})
	}
	return &View{
		OrderedColumns: []string{"value", "file", "directory", "component", "type", "begin", "end"},
		rows:           toReturn,
	}
}

func DirectoryRecursiveView(results *snippets.Results) *View {
	var toReturn []*Row
	snippetsByDirectory := results.SnippetsByDirectory
	allColumns := getDistinctColumnsFromResults(results)
	statsByDirectory := statsByGroup(allColumns, snippetsByDirectory)
	allDirs := make([]string, 0, len(snippetsByDirectory))

	for dir, _ := range snippetsByDirectory {
		allDirs = append(allDirs, dir)
	}

	dirLookup := createDirectoryTree(results.RootDirectory, allDirs)

	for dir, node := range dirLookup {
		subtree := ToPaths(node.Subtree())
		var stats Stats
		for _, subDir := range subtree {
			stats = stats.Merge(statsByDirectory[subDir])
		}
		toReturn = append(toReturn, &Row{
			Data: statsToRowData(dir, stats),
		})
	}
	columnsToReturn := []string{"name"}
	for _, column := range allColumns {
		columnsToReturn = append(columnsToReturn, column)
	}
	return &View{
		OrderedColumns: columnsToReturn,
		rows:           toReturn,
	}
}

func GenericView(allColumns []string, group snippets.SnippetGroup) *View {
	var toReturn []*Row
	for groupItem, groupedSnippets := range group {
		stats := snippetsToStats(allColumns, groupedSnippets)
		data := statsToRowData(groupItem, stats)
		addAbstractness(data, stats)
		toReturn = append(toReturn, &Row{
			Data: data,
		})
	}

	columnsToReturn := []string{"name"}
	if slices.Contains(allColumns, snippets.AbstractType) {
		columnsToReturn = append(columnsToReturn, "abstractness")
	}
	for _, column := range allColumns {
		columnsToReturn = append(columnsToReturn, column)
	}
	return &View{
		OrderedColumns: columnsToReturn,
		rows:           toReturn,
	}
}
func addAbstractness(data map[string]interface{}, stats Stats) {
	if _, hasAbstractTypes := data[snippets.AbstractType]; hasAbstractTypes {
		abstractTypes, types := stats[snippets.AbstractType], stats[snippets.Type]
		abstractness := math.Max(0, math.Min(1, float64(abstractTypes)/float64(types)))
		data[Abstractness] = abstractness
	}
}

func statsToRowData(name string, stats Stats) map[string]interface{} {
	toReturn := make(map[string]interface{}, len(stats)+1)
	toReturn["name"] = name
	for k, v := range stats {
		toReturn[k] = v
	}
	return toReturn
}

func snippetsToStats(allStats []string, allSnippets []*snippets.Snippet) Stats {
	stats := Stats{}
	all := snippets.GroupSnippetsBy(allSnippets, snippets.ByType)

	for _, stat := range allStats {
		snippetsForType := all[stat]
		statToAdd := Stats{stat: len(snippetsForType)}

		stats = stats.Merge(statToAdd)
	}
	return stats
}

func statsByGroup(allStats []string, group snippets.SnippetGroup) map[string]Stats {
	toReturn := map[string]Stats{}
	for groupItem, snippets := range group {
		toReturn[groupItem] = snippetsToStats(allStats, snippets)
	}
	return toReturn
}

func getDistinctColumnsFromResults(results *snippets.Results) []string {
	var toReturn []string
	for theType, _ := range results.SnippetsByType {
		toReturn = append(toReturn, theType)
	}
	sort.Strings(toReturn)
	return toReturn
}
