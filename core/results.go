package core

import (
	"fmt"
	"github.com/archstats/archstats/core/component"
	definitions2 "github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/core/walker"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"strings"
	"sync"
)

// Results represents the results of an analysis in pre-aggregated form.
type Results struct {
	RootDirectory string

	Snippets            []*file.Snippet
	SnippetsByFile      file.SnippetGroup
	SnippetsByDirectory file.SnippetGroup
	SnippetsByComponent file.SnippetGroup
	SnippetsByType      file.SnippetGroup

	Stats            *stats.Stats
	StatsByFile      stats.StatsGroup
	StatsByDirectory stats.StatsGroup
	StatsByComponent stats.StatsGroup

	StatRecords       []*stats.Record
	StatRecordsByFile map[string][]*stats.Record

	Connections     []*component.Connection
	ConnectionsFrom map[string][]*component.Connection
	ConnectionsTo   map[string][]*component.Connection

	FileToComponent map[string]string
	FileToDirectory map[string]string

	ComponentToFiles map[string][]string
	DirectoryToFiles map[string][]string

	ComponentGraph *component.Graph

	Views []*View

	accumulators  *stats.StatAccumulator
	views         map[string]*ViewFactory
	definitions   map[string]*definitions2.Definition
	renderedViews map[string]*View
}
type groupedSnippets struct {
	all         []*file.Snippet
	byDirectory file.SnippetGroup
	byComponent file.SnippetGroup
	byFile      file.SnippetGroup
	byType      file.SnippetGroup
}

func breakSnippetsIntoGroups(fileResults []*file.Results) groupedSnippets {
	allSnippets := lo.FlatMap(fileResults, func(fileResult *file.Results, idx int) []*file.Snippet {
		return fileResult.Snippets
	})
	allSnippetGroups := file.MultiGroupSnippetsBy(allSnippets, map[string]file.GroupSnippetByFunc{
		"ByDirectory": file.ByDirectory,
		"ByComponent": file.ByComponent,
		"ByFile":      file.ByFile,
		"ByType":      file.ByType,
	})

	snippetsByComponent, snippetsByType, snippetsByFile, snippetsByDirectory :=
		allSnippetGroups["ByComponent"], allSnippetGroups["ByType"], allSnippetGroups["ByFile"], allSnippetGroups["ByDirectory"]

	return groupedSnippets{
		all:         allSnippets,
		byDirectory: snippetsByDirectory,
		byComponent: snippetsByComponent,
		byFile:      snippetsByFile,
		byType:      snippetsByType,
	}
}

func aggregateSnippetsAndStatsIntoResults(settings *analyzer, fileResults []*file.Results) *Results {
	rootPath, theAccumulator := settings.rootPath, settings.accumulators
	snippets := breakSnippetsIntoGroups(fileResults)
	var statRecordsByFile = lo.SliceToMap(fileResults, func(fileResult *file.Results) (string, []*stats.Record) {
		return fileResult.Name, fileResult.Stats
	})
	statsByFile := lo.MapValues(statRecordsByFile, func(statRecords []*stats.Record, _ string) *stats.Stats {
		return theAccumulator.Merge(statRecords)
	})
	componentToFiles := lo.MapValues(snippets.byComponent, func(snippets []*file.Snippet, _ string) []string {
		return lo.Uniq(lo.Map(snippets, func(snippet *file.Snippet, idx int) string {
			return snippet.File
		}))
	})
	statsByComponent := lo.MapValues(componentToFiles, func(files []string, component string) *stats.Stats {
		var stats_ []*stats.Record
		for _, file := range files {
			stats_ = append(stats_, statRecordsByFile[file]...)
		}
		stats_ = append(stats_, &stats.Record{
			StatType: file.FileCount,
			Value:    len(files),
		})
		return theAccumulator.Merge(stats_)
	})

	directoryResults := lo.GroupBy(fileResults, func(item *file.Results) string {
		return item.Name[:strings.LastIndex(item.Name, "/")]
	})
	statsByDirectory := lo.MapValues(directoryResults, func(files []*file.Results, directory string) *stats.Stats {
		var stats_ []*stats.Record
		for _, file := range files {
			stats_ = append(stats_, file.Stats...)
		}
		stats_ = append(stats_, &stats.Record{
			StatType: file.FileCount,
			Value:    len(files),
		})
		return theAccumulator.Merge(stats_)
	})

	allStatRecords := lo.Flatten(lo.MapToSlice(statRecordsByFile, func(file string, statRecords []*stats.Record) []*stats.Record {
		return statRecords
	}))
	allStatRecords = append(allStatRecords, &stats.Record{
		StatType: file.FileCount,
		Value:    len(statRecordsByFile),
	})
	statsTotal := theAccumulator.Merge(allStatRecords)

	fileToComponent := lo.MapValues(snippets.byFile, func(snippets []*file.Snippet, _ string) string {
		return snippets[0].Component
	})
	fileToDirectory := lo.MapValues(statsByFile, func(snippets *stats.Stats, file string) string {
		return file[:strings.LastIndex(file, "/")]
	})
	componentConnections := component.GetConnectionsFromSnippetImports(snippets.byType, snippets.byComponent)
	graph := component.CreateGraph("all", lo.Keys(componentToFiles), componentConnections)
	directoryToFiles := lo.MapValues(snippets.byDirectory, func(snippets []*file.Snippet, _ string) []string {
		return lo.Uniq(lo.Map(snippets, func(snippet *file.Snippet, idx int) string {
			return snippet.File
		}))
	})
	return &Results{
		RootDirectory: rootPath,

		Stats:            statsTotal,
		StatsByFile:      statsByFile,
		StatsByDirectory: statsByDirectory,
		StatsByComponent: statsByComponent,

		StatRecords:       allStatRecords,
		StatRecordsByFile: statRecordsByFile,

		Snippets:            snippets.all,
		SnippetsByDirectory: snippets.byDirectory,
		SnippetsByComponent: snippets.byComponent,
		SnippetsByFile:      snippets.byFile,
		SnippetsByType:      snippets.byType,

		ComponentGraph:  graph,
		Connections:     graph.Connections,
		ConnectionsFrom: graph.ConnectionsFrom,
		ConnectionsTo:   graph.ConnectionsTo,

		FileToComponent:  fileToComponent,
		FileToDirectory:  fileToDirectory,
		ComponentToFiles: componentToFiles,
		DirectoryToFiles: directoryToFiles,

		views:         settings.views,
		definitions:   settings.definitions,
		renderedViews: make(map[string]*View),
		accumulators:  theAccumulator,
	}
}

func (r *Results) CalculateAccumulatedStatRecords(keyToStatRecords map[string][]*stats.Record) stats.StatsGroup {
	return lo.MapValues(keyToStatRecords, func(statRecords []*stats.Record, _ string) *stats.Stats {
		return r.accumulators.Merge(statRecords)
	})
}
func (r *Results) RenderView(viewName string) (*View, error) {
	log.Debug().Msgf("Rendering view '%s'", viewName)
	defer log.Debug().Msgf("Finished rendering view '%s'", viewName)
	if view, ok := r.renderedViews[viewName]; ok {
		return view, nil
	}
	if viewFactory, ok := r.views[viewName]; ok {
		view := viewFactory.CreateViewFunc(r)
		view.Name = viewName
		return view, nil
	} else {
		availableKeys := strings.Join(lo.MapToSlice(r.views, func(k string, v *ViewFactory) string {
			return fmt.Sprintf("'%s'", k)
		}), ", ")
		return nil, fmt.Errorf("no view named '%s', available views: %v", viewName, availableKeys)
	}
}

func (r *Results) GetViewFactories() []*ViewFactory {
	var views []*ViewFactory
	for _, vf := range r.views {
		views = append(views, vf)
	}
	return views
}

func (r *Results) GetDefinitions() map[string]*definitions2.Definition {
	return r.definitions
}
func (r *Results) GetDefinition(str string) *definitions2.Definition {
	return r.definitions[str]
}

func getAllFileResults(rootPath string, fileAnalyzers []FileAnalyzer) []*file.Results {
	var allFileResults []*file.Results

	lock := sync.Mutex{}
	walker.WalkDirectoryConcurrently(rootPath, func(theFile file.File) {
		var currentFileResultsToMerge []*file.Results
		for _, provider := range fileAnalyzers {
			analyzeFile := provider.AnalyzeFile(theFile)
			if analyzeFile != nil {
				currentFileResultsToMerge = append(currentFileResultsToMerge, analyzeFile)
			}
		}
		currentFileResults := mergeFileResults(currentFileResultsToMerge)
		currentFileResults.Name = theFile.Path()
		currentFileResults.Directory = theFile.Path()[:strings.LastIndex(theFile.Path(), "/")]
		file.AddLineNumberAndCharInLineToSnippets(theFile.Content(), currentFileResults.Snippets)
		lock.Lock()
		allFileResults = append(allFileResults, currentFileResults)
		lock.Unlock()
	})
	return allFileResults
}

func mergeFileResults(results []*file.Results) *file.Results {
	newResults := &file.Results{}
	for _, otherResult := range results {
		newResults.Component = otherResult.Component
		newResults.Name = otherResult.Name
		newResults.Directory = otherResult.Directory
		newResults.Stats = append(newResults.Stats, otherResult.Stats...)
		newResults.Snippets = append(newResults.Snippets, otherResult.Snippets...)
	}
	return newResults
}
