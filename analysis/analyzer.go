package analysis

import (
	"errors"
	"github.com/RyanSusana/archstats/walker"
	"github.com/samber/lo"
	"sort"
	"strings"
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

type SnippetProvider interface {
	GetSnippetsFromFile(File) []*Snippet
}

type SnippetEditor interface {
	EditSnippet(current *Snippet)
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

	// Get Snippets and Stats from the files
	snippetProviders := getGenericExtensions[SnippetProvider](allExtensions)
	statProviders := getGenericExtensions[StatProvider](allExtensions)
	allSnippets, allStatsByFile := getStatsAndSnippets(settings.RootPath, snippetProviders, statProviders)

	// Pre-sort the snippets to make sure they are in the same order every time.
	sort.Slice(allSnippets, func(i, j int) bool {
		if allSnippets[i].File != allSnippets[j].File {
			return allSnippets[i].File < allSnippets[j].File
		}
		if allSnippets[i].Begin != allSnippets[j].Begin {
			return allSnippets[i].Begin < allSnippets[j].Begin
		}
		return allSnippets[i].End < allSnippets[j].End
	})

	if len(allSnippets) == 0 {
		return nil, errors.New("could not find any snippets")
	}

	// Edit snippets after they've been identified
	// Used to set the component and directory of a snippet
	snippetEditors := getGenericExtensions[SnippetEditor](allExtensions)
	for _, editor := range snippetEditors {
		for _, snippet := range allSnippets {
			editor.EditSnippet(snippet)
		}
	}

	// Aggregate Snippets and Stats into Results
	results := aggregateResults(settings.RootPath, allSnippets, allStatsByFile)

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

func getExtensionsFromSettings(settings *Settings) []Extension {
	allExtensions := []Extension{
		&fileMarker{},
		&rootPathStripper{},
	}

	for _, extension := range settings.Extensions {
		allExtensions = append(allExtensions, extension)
	}
	return allExtensions
}

func getStatsAndSnippets(rootPath string, snippetProviders []SnippetProvider, statProviders []StatProvider) ([]*Snippet, StatsGroup) {
	var allSnippets []*Snippet
	var allStatsByFile StatsGroup

	lock := sync.Mutex{}
	walker.WalkDirectoryConcurrently(rootPath, func(file walker.OpenedFile) {
		var foundSnippets []*Snippet
		var foundStats []*Stats
		for _, provider := range snippetProviders {
			foundSnippets = append(foundSnippets, provider.GetSnippetsFromFile(file)...)
		}
		foundStats = append(foundStats, snippetsToStats(foundSnippets))
		for _, provider := range statProviders {
			statsToAdd := provider.GetStatsFromFile(file)
			foundStats = append(foundStats, statsToAdd)
		}
		lock.Lock()
		allSnippets = append(allSnippets, foundSnippets...)
		allStatsByFile[file.Path()] = MergeMultipleStats(foundStats)
		lock.Unlock()
	})
	return allSnippets, allStatsByFile
}

func aggregateResults(rootPath string, allSnippets []*Snippet, statsByFile StatsGroup) *Results {
	//set Directory name for each Snippet
	setDirectories(allSnippets)
	setComponents(allSnippets)

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

	statsByComponent := lo.MapValues(snippetsByComponent, func(snippets []*Snippet, _ string) *Stats {
		return snippetsToStats(snippets)
	})
	statsByDirectory := lo.MapValues(snippetsByDirectory, func(snippets []*Snippet, _ string) *Stats {
		return snippetsToStats(snippets)
	})
	statsTotal := MergeMultipleStats(lo.MapToSlice(statsByFile, func(_ string, stats *Stats) *Stats {
		return stats
	}))

	fileToComponent := lo.MapValues(snippetsByFile, func(snippets []*Snippet, _ string) string {
		return snippets[0].Component
	})

	componentToFiles := lo.MapValues(snippetsByComponent, func(snippets []*Snippet, _ string) []string {
		return lo.Map(snippets, func(snippet *Snippet, idx int) string {
			return snippet.File
		})
	})

	directoryToFiles := lo.MapValues(snippetsByDirectory, func(snippets []*Snippet, _ string) []string {
		return lo.Map(snippets, func(snippet *Snippet, idx int) string {
			return snippet.File
		})
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

func setDirectories(s []*Snippet) {
	for _, snippet := range s {
		fileName := snippet.File
		snippet.Directory = fileName[:strings.LastIndex(fileName, "/")]
	}
}

func setComponents(s []*Snippet) {
	componentDeclarations := lo.Filter(s, func(snippet *Snippet, idx int) bool {
		return snippet.Type == ComponentDeclaration
	})
	snippetsByFile := lo.GroupBy(s, ByFile)
	componentDeclarationsByFile := lo.GroupBy(componentDeclarations, ByFile)

	for fileName, componentDeclarationSnippets := range componentDeclarationsByFile {
		if len(componentDeclarationSnippets) == 0 {
			continue
		}
		theComponent := componentDeclarationSnippets[0].Value
		snippets := snippetsByFile[fileName]
		for _, theSnippet := range snippets {
			theSnippet.Component = theComponent
		}
	}
}

func getConnections(snippetsByType SnippetGroup, snippetsByComponent SnippetGroup) []*ComponentConnection {
	var toReturn []*ComponentConnection
	from := snippetsByType[ComponentImport]
	for _, snippet := range from {
		if _, componentExistsInCodebase := snippetsByComponent[snippet.Value]; componentExistsInCodebase {
			toReturn = append(toReturn, &ComponentConnection{
				From: snippet.Component,
				To:   snippet.Value,
				File: snippet.File,
			})
		}
	}
	return toReturn
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
