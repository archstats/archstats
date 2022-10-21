package analysis

import (
	"github.com/RyanSusana/archstats/walker"
	"github.com/samber/lo"
	"sync"
)

type Settings struct {
	RootPath   string
	Extensions []Extension
}

type Extension interface{}

type Initializable interface {
	Init(settings *Settings)
}

type FileResultsEditor interface {
	EditFileResults(all []*FileResults)
}

type ResultEditor interface {
	EditResults(results *Results)
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

	ComponentToFiles map[string][]string
	DirectoryToFiles map[string][]string
}

// Analyze analyzes the given root directory and returns the results.
func Analyze(settings *Settings) (*Results, error) {
	allExtensions := getExtensionsFromSettings(settings)

	// Initialize extensions that depend on settings
	initializeExtensions(settings, allExtensions)

	// Get Snippets and OnlyStats from the files
	fileScanners := getGenericExtensions[FileAnalyzer](allExtensions)
	fileResults := getAllFileResults(settings.RootPath, fileScanners)

	// Edit file results
	// Used to set the component and directory of a snippet
	fileResultsEditors := getGenericExtensions[FileResultsEditor](allExtensions)
	for _, editor := range fileResultsEditors {
		editor.EditFileResults(fileResults)
	}

	// Aggregate Snippets and OnlyStats into Results
	results := aggregateResults(settings.RootPath, fileResults)

	// Edit results after they've been aggregated
	resultEditors := getGenericExtensions[ResultEditor](allExtensions)
	for _, editor := range resultEditors {
		editor.EditResults(results)
	}

	return results, nil
}

func initializeExtensions(settings *Settings, allExtensions []Extension) {
	initializables := getGenericExtensions[Initializable](allExtensions)

	for _, initializable := range initializables {
		initializable.Init(settings)
	}
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
		currentFileResults := MergeFileResults(currentFileResultsToMerge)
		currentFileResults.Name = file.Path()
		lock.Lock()
		allFileResults = append(allFileResults, currentFileResults)
		lock.Unlock()
	})
	return allFileResults
}

func aggregateResults(rootPath string, fileResults []*FileResults) *Results {
	allSnippets := lo.FlatMap(fileResults, func(fileResult *FileResults, idx int) []*Snippet {
		return fileResult.Snippets
	})

	statsByFile := lo.SliceToMap(fileResults, func(fileResult *FileResults) (string, *Stats) {
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

	statsByComponent := lo.MapValues(snippetsByComponent, func(snippets []*Snippet, component string) *Stats {
		stats := SnippetsToStats(snippets)
		return MergeMultipleStats([]*Stats{
			{FileCount: len(componentToFiles[component])},
			stats,
		})
	})
	statsByDirectory := lo.MapValues(snippetsByDirectory, func(snippets []*Snippet, directory string) *Stats {
		stats := SnippetsToStats(snippets)
		return MergeMultipleStats([]*Stats{
			{FileCount: len(directoryToFiles[directory])},
			stats,
		})
	})
	statsTotal := MergeMultipleStats(lo.MapToSlice(statsByFile, func(_ string, stats *Stats) *Stats {
		return MergeMultipleStats([]*Stats{
			{FileCount: 1},
			stats,
		})
	}))

	fileToComponent := lo.MapValues(snippetsByFile, func(snippets []*Snippet, _ string) string {
		return snippets[0].Component
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
		ComponentToFiles: componentToFiles,
		DirectoryToFiles: directoryToFiles,
	}
}

func getExtensionsFromSettings(settings *Settings) []Extension {
	allExtensions := []Extension{
		&componentLinker{},
		&rootPathStripper{},
	}

	for _, extension := range settings.Extensions {
		allExtensions = append(allExtensions, extension)
	}
	return allExtensions
}
func getGenericExtensions[K Extension](extensions []Extension) []K {
	var toReturn []K
	for _, extension := range extensions {

		editor, isEditor := extension.(K)
		if isEditor {
			toReturn = append(toReturn, editor)
		}
	}
	return toReturn
}
