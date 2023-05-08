package analysis

import (
	"fmt"
	"github.com/RyanSusana/archstats/walker"
	"github.com/samber/lo"
	"strings"
	"sync"
)

// Analyze analyzes the given root directory and returns the results.
func Analyze(settings *analyzer) (*Results, error) {

	// Initialize extensions
	for _, extension := range settings.extensions {
		err := extension.Init(settings)
		if err != nil {
			return nil, err
		}
	}

	// Get Snippets and OnlyStats from the files
	fileResults := getAllFileResults(settings.rootPath, settings.fileAnalyzers)

	// Edit file results
	// Used to set the component and directory of a snippet
	for _, editor := range settings.fileResultsEditors {
		editor.EditFileResults(fileResults)
	}

	// Aggregate Snippets and OnlyStats into Results
	results := aggregateResults(settings, fileResults)

	// Edit results after they've been aggregated

	for _, editor := range settings.resultsEditors {
		editor.EditResults(results)
	}

	return results, nil
}

// Results represents the results of an analysis in pre-aggregated form.
type Results struct {
	RootDirectory string

	Snippets            []*Snippet
	SnippetsByFile      SnippetGroup
	SnippetsByDirectory SnippetGroup
	SnippetsByComponent SnippetGroup
	SnippetsByType      SnippetGroup

	Stats            *Stats
	StatsByFile      StatsGroup
	StatsByDirectory StatsGroup
	StatsByComponent StatsGroup

	Connections     []*ComponentConnection
	ConnectionsFrom map[string][]*ComponentConnection
	ConnectionsTo   map[string][]*ComponentConnection

	FileToComponent map[string]string
	FileToDirectory map[string]string

	ComponentToFiles map[string][]string
	DirectoryToFiles map[string][]string

	ComponentGraph *ComponentGraph

	views map[string]*ViewFactory
}

func aggregateResults(settings *analyzer, fileResults []*FileResults) *Results {

	rootPath, theAccumulator := settings.rootPath, settings.accumulator
	allSnippets := lo.FlatMap(fileResults, func(fileResult *FileResults, idx int) []*Snippet {
		return fileResult.Snippets
	})

	statRecordsByFile := lo.SliceToMap(fileResults, func(fileResult *FileResults) (string, []*StatRecord) {
		return fileResult.Name, fileResult.Stats
	})

	allSnippetGroups := MultiGroupSnippetsBy(allSnippets, map[string]GroupSnippetByFunc{
		"ByDirectory": ByDirectory,
		"ByComponent": ByComponent,
		"ByFile":      ByFile,
		"ByType":      ByType,
	})

	snippetsByComponent, snippetsByType, snippetsByFile, snippetsByDirectory :=
		allSnippetGroups["ByComponent"], allSnippetGroups["ByType"], allSnippetGroups["ByFile"], allSnippetGroups["ByDirectory"]

	componentConnections := getConnections(snippetsByType, snippetsByComponent)
	componentConnectionsByFrom := lo.GroupBy(componentConnections, func(connection *ComponentConnection) string {
		return connection.From
	})
	componentConnectionsByTo := lo.GroupBy(componentConnections, func(connection *ComponentConnection) string {
		return connection.To
	})

	componentToFiles := lo.MapValues(snippetsByComponent, func(snippets []*Snippet, _ string) []string {
		return lo.Uniq(lo.Map(snippets, func(snippet *Snippet, idx int) string {
			return snippet.File
		}))
	})

	directoryToFiles := lo.MapValues(snippetsByDirectory, func(snippets []*Snippet, _ string) []string {
		return lo.Uniq(lo.Map(snippets, func(snippet *Snippet, idx int) string {
			return snippet.File
		}))
	})

	statsByFile := lo.MapValues(statRecordsByFile, func(statRecords []*StatRecord, _ string) *Stats {
		return theAccumulator.merge(statRecords)
	})

	statsByComponent := lo.MapValues(componentToFiles, func(files []string, component string) *Stats {
		var stats []*StatRecord
		for _, file := range files {
			stats = append(stats, statRecordsByFile[file]...)
		}
		stats = append(stats, &StatRecord{
			StatType: FileCount,
			Value:    len(files),
		})
		return theAccumulator.merge(stats)
	})

	directoryResults := lo.GroupBy(fileResults, func(item *FileResults) string {
		return item.Name[:strings.LastIndex(item.Name, "/")]
	})

	statsByDirectory := lo.MapValues(directoryResults, func(files []*FileResults, directory string) *Stats {
		var stats []*StatRecord
		for _, file := range files {
			stats = append(stats, file.Stats...)
		}
		stats = append(stats, &StatRecord{
			StatType: FileCount,
			Value:    len(files),
		})
		return theAccumulator.merge(stats)
	})

	allStatRecords := lo.Flatten(lo.MapToSlice(statRecordsByFile, func(file string, statRecords []*StatRecord) []*StatRecord {
		return statRecords
	}))
	allStatRecords = append(allStatRecords, &StatRecord{
		StatType: FileCount,
		Value:    len(statRecordsByFile),
	})
	statsTotal := theAccumulator.merge(allStatRecords)

	fileToComponent := lo.MapValues(snippetsByFile, func(snippets []*Snippet, _ string) string {
		return snippets[0].Component
	})
	fileToDirectory := lo.MapValues(statsByFile, func(snippets *Stats, file string) string {
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
		ComponentGraph:   createComponentGraph(componentConnections),

		views: settings.views,
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

func (r *Results) GetAllViewFactories() []*ViewFactory {
	var views []*ViewFactory
	for _, vf := range r.views {
		views = append(views, vf)
	}
	return views
}

func getAllFileResults(rootPath string, snippetProviders []FileAnalyzer) []*FileResults {
	var allFileResults []*FileResults

	lock := sync.Mutex{}
	walker.WalkDirectoryConcurrently(rootPath, func(file walker.OpenedFile) {
		var currentFileResultsToMerge []*FileResults
		for _, provider := range snippetProviders {
			analyzeFile := provider.AnalyzeFile(file)
			if analyzeFile != nil {
				currentFileResultsToMerge = append(currentFileResultsToMerge, analyzeFile)
			}
		}
		currentFileResults := mergeFileResults(currentFileResultsToMerge)
		currentFileResults.Name = file.Path()
		lock.Lock()
		allFileResults = append(allFileResults, currentFileResults)
		lock.Unlock()
	})
	return allFileResults
}
