package main

import (
	"analyzer/snippets"
	"fmt"
)

// getRowsFromResults returns the list of rows based on the input command from the CLI
func getRowsFromResults(command string, results *snippets.Results) ([]*Row, error) {
	views := map[string]ViewFunction{
		"components":            ComponentView,
		"files":                 FileView,
		"directories":           DirectoryView,
		"directories-recursive": DirectoryRecursiveView,
	}

	if view, isAnAvailableView := views[command]; isAnAvailableView {
		return view(results), nil
	} else {
		return nil, fmt.Errorf("%s is not a recognized view", command)
	}
}

type ViewFunction func(results *snippets.Results) []*Row

type Row struct {
	Name string
	Data map[string]interface{}
}

func DirectoryView(results *snippets.Results) []*Row {
	return GenericView(getDistinctStatsFromResults(results), results.SnippetsByDirectory)
}
func ComponentView(results *snippets.Results) []*Row {
	return GenericView(getDistinctStatsFromResults(results), results.SnippetsByComponent)
}
func FileView(results *snippets.Results) []*Row {
	return GenericView(getDistinctStatsFromResults(results), results.SnippetsByFile)
}

func DirectoryRecursiveView(results *snippets.Results) []*Row {
	var toReturn []*Row
	snippetsByDirectory := results.SnippetsByDirectory
	statsByDirectory := statsByGroup(getDistinctStatsFromResults(results), snippetsByDirectory)
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
			Name: dir,
			Data: statsToRowData(stats),
		})
	}
	return toReturn
}

func GenericView(allStats []string, group snippets.SnippetGroup) []*Row {
	var toReturn []*Row
	for groupItem, snippets := range group {
		stats := snippetsToStats(allStats, snippets)
		data := statsToRowData(stats)
		toReturn = append(toReturn, &Row{
			Name: groupItem,
			Data: data,
		})
	}
	return toReturn
}

func statsToRowData(stats Stats) map[string]interface{} {
	toReturn := make(map[string]interface{}, len(stats))
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

func getDistinctStatsFromRows(all []*Row) []string {
	allStats := map[string]bool{}
	for _, row := range all {
		for s, _ := range row.Data {
			allStats[s] = true
		}
	}
	keys := make([]string, len(allStats))
	i := 0
	for k := range allStats {
		keys[i] = k
		i++
	}
	return keys
}

func getDistinctStatsFromResults(results *snippets.Results) []string {
	var toReturn []string
	for theType, _ := range results.SnippetsByType {
		toReturn = append(toReturn, theType)
	}
	return toReturn
}
