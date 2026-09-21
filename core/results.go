package core

import (
	"fmt"
	"github.com/archstats/archstats/core/component"
	definitions2 "github.com/archstats/archstats/core/definitions"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/stats"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/core/walker"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"path/filepath"
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

	StatRecords       []*stats.Record
	StatRecordsByFile map[string][]*stats.Record

	// Connections holds the edges that exist when the program runs, and is
	// what every coupling metric is computed from. AllConnections also holds
	// the edges the compiler erases -- TypeScript's `import type` -- which
	// are real dependencies on a shape and would inflate coupling by an
	// eighth on a codebase like LibreChat if counted.
	Connections     []*component.Connection
	AllConnections  []*component.Connection
	ConnectionsFrom map[string][]*component.Connection
	ConnectionsTo   map[string][]*component.Connection

	FileToComponent map[string]string
	FileToDirectory map[string]string
	FileToModule    map[string]string

	ComponentToFiles map[string][]string
	DirectoryToFiles map[string][]string
	ModuleToFiles    map[string][]string

	// The named things in the codebase: types, functions, modules. Folded
	// across files, so one C# partial class spread over three files is one
	// unit with three files rather than three units. See core/unit.
	Units       []*unit.Unit
	UnitsByKind map[string][]*unit.Unit
	UnitsByFile map[string][]*unit.Unit
	UnitByID    map[string]*unit.Unit

	// What the project declares it builds and publishes, read from its own
	// manifests. Empty for a project that declares nothing, which is most
	// single-package repositories and is not an error. See core/module.
	Modules *module.Map

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
	directoryToFiles := mapDirectoryToFiles(lo.Keys(snippets.byFile))
	allStatRecords := lo.Flatten(lo.MapToSlice(statRecordsByFile, func(file string, statRecords []*stats.Record) []*stats.Record {
		return statRecords
	}))
	allStatRecords = append(allStatRecords, &stats.Record{
		StatType: file.FileCount,
		Value:    len(statRecordsByFile),
	})

	fileToComponent := lo.MapValues(snippets.byFile, func(snippets []*file.Snippet, _ string) string {
		return snippets[0].Component
	})
	fileToDirectory := lo.MapValues(statsByFile, func(snippets *stats.Stats, file string) string {
		return file[:strings.LastIndex(file, "/")]
	})
	// Modules are read from the manifests among the files the walker found,
	// so .gitignore and .archstatsignore are honoured for free (ADR 0012).
	allFileNames := lo.Map(fileResults, func(fr *file.Results, _ int) string { return fr.Name })
	moduleMap := module.ReadFrom(rootPath, allFileNames)
	fileToModule := make(map[string]string, len(allFileNames))
	moduleToFiles := make(map[string][]string)
	for _, name := range allFileNames {
		if mod := moduleMap.Of(name); mod != nil {
			fileToModule[name] = mod.Name
			moduleToFiles[mod.Name] = append(moduleToFiles[mod.Name], name)
		}
	}

	// Units are folded across files before anything reads them: a partial
	// class declared in three files is one unit, and nopCommerce has 1,567
	// of them.
	var rawUnits []*unit.Unit
	for _, fr := range fileResults {
		rawUnits = append(rawUnits, fr.Units...)
	}
	allUnits := unit.Merge(rawUnits)
	unitsByKind := map[string][]*unit.Unit{}
	unitsByFile := map[string][]*unit.Unit{}
	unitByID := make(map[string]*unit.Unit, len(allUnits))
	for _, u := range allUnits {
		// A unit's component and module come from the files it is declared
		// in, which are only known once component resolution has run.
		for _, f := range u.Files {
			if u.Component == "" {
				u.Component = fileToComponent[f]
			}
			if u.Module == "" {
				u.Module = fileToModule[f]
			}
			unitsByFile[f] = append(unitsByFile[f], u)
		}
		unitsByKind[u.Kind] = append(unitsByKind[u.Kind], u)
		unitByID[u.ID] = u
	}

	allConnections := component.GetConnectionsFromSnippetImports(snippets.byType, snippets.byComponent)
	graph := component.CreateGraph("all", lo.Keys(componentToFiles), component.RuntimeOnly(allConnections))

	return &Results{
		RootDirectory: rootPath,

		StatRecords:       allStatRecords,
		StatRecordsByFile: statRecordsByFile,

		Snippets:            snippets.all,
		SnippetsByDirectory: snippets.byDirectory,
		SnippetsByComponent: snippets.byComponent,
		SnippetsByFile:      snippets.byFile,
		SnippetsByType:      snippets.byType,

		ComponentGraph:  graph,
		Connections:     graph.Connections,
		AllConnections:  allConnections,
		ConnectionsFrom: graph.ConnectionsFrom,
		ConnectionsTo:   graph.ConnectionsTo,

		Units:       allUnits,
		UnitsByKind: unitsByKind,
		UnitsByFile: unitsByFile,
		UnitByID:    unitByID,

		FileToComponent:  fileToComponent,
		FileToDirectory:  fileToDirectory,
		FileToModule:     fileToModule,
		ComponentToFiles: componentToFiles,
		DirectoryToFiles: directoryToFiles,
		ModuleToFiles:    moduleToFiles,
		Modules:          moduleMap,

		views:         settings.views,
		definitions:   settings.definitions,
		renderedViews: make(map[string]*View),
		accumulators:  theAccumulator,
	}
}

func (r *Results) Calculate(records []*stats.Record) *stats.Stats {
	return r.accumulators.Merge(records)
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

func getAllFileResults(rootPath string, fileAnalyzers []FileAnalyzer) ([]*file.Results, error) {
	var allFileResults []*file.Results

	lock := sync.Mutex{}
	err := walker.WalkDirectoryConcurrently(rootPath, func(theFile file.File) {
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
	if err != nil {
		return nil, err
	}
	return allFileResults, nil
}

func mergeFileResults(results []*file.Results) *file.Results {
	newResults := &file.Results{}
	for _, otherResult := range results {
		newResults.Component = otherResult.Component
		newResults.Name = otherResult.Name
		newResults.Directory = otherResult.Directory
		newResults.Stats = append(newResults.Stats, otherResult.Stats...)
		newResults.Snippets = append(newResults.Snippets, otherResult.Snippets...)
		newResults.Units = append(newResults.Units, otherResult.Units...)
	}
	return newResults
}

func mapDirectoryToFiles(files []string) map[string][]string {
	directoryFiles := make(map[string][]string)

	for _, f := range files {
		for _, dir := range getParentDirectories(f) {
			directoryFiles[dir] = append(directoryFiles[dir], f)
		}
	}
	return directoryFiles
}

// getParentDirectories returns all parent directories of the given path.
func getParentDirectories(path string) []string {
	var dirs []string
	dir := filepath.Dir(path)

	for dir != "." && dir != "/" {
		dirs = append(dirs, dir)
		dir = filepath.Dir(dir)
	}

	return dirs
}
