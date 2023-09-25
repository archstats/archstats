package analysis

import (
	"fmt"
	"github.com/RyanSusana/archstats/analysis/component"
	definitions2 "github.com/RyanSusana/archstats/analysis/definitions"
	"github.com/RyanSusana/archstats/analysis/file"
	"github.com/RyanSusana/archstats/analysis/walker"
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

	Stats            *file.Stats
	StatsByFile      file.StatsGroup
	StatsByDirectory file.StatsGroup
	StatsByComponent file.StatsGroup

	Connections     []*component.Connection
	ConnectionsFrom map[string][]*component.Connection
	ConnectionsTo   map[string][]*component.Connection

	FileToComponent map[string]string
	FileToDirectory map[string]string

	ComponentToFiles map[string][]string
	DirectoryToFiles map[string][]string

	ComponentGraph *component.Graph

	Views []*View

	views       map[string]*ViewFactory
	definitions map[string]*definitions2.Definition
}

func aggregateSnippetsAndStatsIntoResults(settings *analyzer, fileResults []*file.Results) *Results {

	rootPath, theAccumulator := settings.rootPath, settings.accumulator
	allSnippets := lo.FlatMap(fileResults, func(fileResult *file.Results, idx int) []*file.Snippet {
		return fileResult.Snippets
	})

	statRecordsByFile := lo.SliceToMap(fileResults, func(fileResult *file.Results) (string, []*file.StatRecord) {
		return fileResult.Name, fileResult.Stats
	})

	allSnippetGroups := file.MultiGroupSnippetsBy(allSnippets, map[string]file.GroupSnippetByFunc{
		"ByDirectory": file.ByDirectory,
		"ByComponent": file.ByComponent,
		"ByFile":      file.ByFile,
		"ByType":      file.ByType,
	})

	snippetsByComponent, snippetsByType, snippetsByFile, snippetsByDirectory :=
		allSnippetGroups["ByComponent"], allSnippetGroups["ByType"], allSnippetGroups["ByFile"], allSnippetGroups["ByDirectory"]

	componentConnections := component.GetConnections(snippetsByType, snippetsByComponent)
	componentConnectionsByFrom := lo.GroupBy(componentConnections, func(connection *component.Connection) string {
		return connection.From
	})
	componentConnectionsByTo := lo.GroupBy(componentConnections, func(connection *component.Connection) string {
		return connection.To
	})

	componentToFiles := lo.MapValues(snippetsByComponent, func(snippets []*file.Snippet, _ string) []string {
		return lo.Uniq(lo.Map(snippets, func(snippet *file.Snippet, idx int) string {
			return snippet.File
		}))
	})

	directoryToFiles := lo.MapValues(snippetsByDirectory, func(snippets []*file.Snippet, _ string) []string {
		return lo.Uniq(lo.Map(snippets, func(snippet *file.Snippet, idx int) string {
			return snippet.File
		}))
	})

	statsByFile := lo.MapValues(statRecordsByFile, func(statRecords []*file.StatRecord, _ string) *file.Stats {
		return theAccumulator.merge(statRecords)
	})

	statsByComponent := lo.MapValues(componentToFiles, func(files []string, component string) *file.Stats {
		var stats []*file.StatRecord
		for _, file := range files {
			stats = append(stats, statRecordsByFile[file]...)
		}
		stats = append(stats, &file.StatRecord{
			StatType: file.FileCount,
			Value:    len(files),
		})
		return theAccumulator.merge(stats)
	})

	directoryResults := lo.GroupBy(fileResults, func(item *file.Results) string {
		return item.Name[:strings.LastIndex(item.Name, "/")]
	})

	statsByDirectory := lo.MapValues(directoryResults, func(files []*file.Results, directory string) *file.Stats {
		var stats []*file.StatRecord
		for _, file := range files {
			stats = append(stats, file.Stats...)
		}
		stats = append(stats, &file.StatRecord{
			StatType: file.FileCount,
			Value:    len(files),
		})
		return theAccumulator.merge(stats)
	})

	allStatRecords := lo.Flatten(lo.MapToSlice(statRecordsByFile, func(file string, statRecords []*file.StatRecord) []*file.StatRecord {
		return statRecords
	}))
	allStatRecords = append(allStatRecords, &file.StatRecord{
		StatType: file.FileCount,
		Value:    len(statRecordsByFile),
	})
	statsTotal := theAccumulator.merge(allStatRecords)

	fileToComponent := lo.MapValues(snippetsByFile, func(snippets []*file.Snippet, _ string) string {
		return snippets[0].Component
	})
	fileToDirectory := lo.MapValues(statsByFile, func(snippets *file.Stats, file string) string {
		return file[:strings.LastIndex(file, "/")]
	})
	return &Results{
		RootDirectory: rootPath,

		Stats:            statsTotal,
		StatsByFile:      statsByFile,
		StatsByDirectory: statsByDirectory,
		StatsByComponent: statsByComponent,

		Snippets:            allSnippets,
		SnippetsByDirectory: snippetsByDirectory,
		SnippetsByComponent: snippetsByComponent,
		SnippetsByFile:      snippetsByFile,
		SnippetsByType:      snippetsByType,

		Connections:     componentConnections,
		ConnectionsFrom: componentConnectionsByFrom,
		ConnectionsTo:   componentConnectionsByTo,

		FileToComponent:  fileToComponent,
		FileToDirectory:  fileToDirectory,
		ComponentToFiles: componentToFiles,
		DirectoryToFiles: directoryToFiles,
		ComponentGraph:   component.CreateGraph(componentConnections),

		views:       settings.views,
		definitions: settings.definitions,
	}
}
func (r *Results) RenderView(viewName string) (*View, error) {
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
		newResults.Stats = append(newResults.Stats, otherResult.Stats...)
		newResults.Snippets = append(newResults.Snippets, otherResult.Snippets...)
	}
	return newResults
}
