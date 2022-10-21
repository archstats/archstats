package analysis

import (
	"github.com/RyanSusana/archstats/walker"
	"github.com/samber/lo"
	"sync"
)

// Analyze analyzes the given root directory and returns the results.
func Analyze(settings *settings) (*Results, error) {
	allExtensions := getExtensionsFromSettings(settings)

	// Initialize extensions that depend on settings
	initializeExtensions(settings, allExtensions)

	// Get Snippets and OnlyStats from the files
	fileScanners := getGenericExtensions[FileAnalyzer](allExtensions)
	fileResults := getAllFileResults(settings.rootPath, fileScanners)

	// Edit file results
	// Used to set the component and directory of a snippet
	fileResultsEditors := getGenericExtensions[FileResultsEditor](allExtensions)
	for _, editor := range fileResultsEditors {
		editor.EditFileResults(fileResults)
	}

	// Aggregate Snippets and OnlyStats into Results
	results := aggregateResults(settings.rootPath, fileResults)

	// Edit results after they've been aggregated
	resultEditors := getGenericExtensions[ResultsEditor](allExtensions)
	for _, editor := range resultEditors {
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
}

func initializeExtensions(settings *settings, allExtensions []Extension) {
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
	m := &merger{}

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

	fileToComponent := lo.MapValues(snippetsByFile, func(snippets []*Snippet, _ string) string {
		return snippets[0].Component
	})
	fileToDirectory := lo.MapValues(snippetsByFile, func(snippets []*Snippet, _ string) string {
		return snippets[0].Directory
	})

	statsByFile := lo.MapValues(statRecordsByFile, func(statRecords []*StatRecord, _ string) *Stats {
		return m.merge(statRecords)
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
		return m.merge(stats)
	})

	statsByDirectory := lo.MapValues(directoryToFiles, func(files []string, directory string) *Stats {
		var stats []*StatRecord
		for _, file := range files {
			stats = append(stats, statRecordsByFile[file]...)
		}
		stats = append(stats, &StatRecord{
			StatType: FileCount,
			Value:    len(files),
		})
		return m.merge(stats)
	})

	allStatRecords := lo.Flatten(lo.MapToSlice(statRecordsByFile, func(file string, statRecords []*StatRecord) []*StatRecord {
		return statRecords
	}))
	allStatRecords = append(allStatRecords, &StatRecord{
		StatType: FileCount,
		Value:    len(statRecordsByFile),
	})
	statsTotal := m.merge(allStatRecords)

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
	}
}

func getExtensionsFromSettings(settings *settings) []Extension {
	allExtensions := []Extension{
		&componentLinker{},
		&rootPathStripper{},
	}

	for _, extension := range settings.extensions {
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
