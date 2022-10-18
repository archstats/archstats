package snippets

import (
	"errors"
	"github.com/RyanSusana/archstats/walker"
	"sort"
	"strings"
	"sync"
)

const (
	ComponentDeclaration = "component_declaration"
	ComponentImport      = "component_import"
	AbstractType         = "abstract_type"
	Type                 = "type"
)

type Results struct {
	RootDirectory       string
	Snippets            []*Snippet
	SnippetsByDirectory SnippetGroup
	SnippetsByComponent SnippetGroup
	SnippetsByFile      SnippetGroup
	SnippetsByType      SnippetGroup
	Connections         []*ComponentConnection
	ConnectionsFrom     map[string][]*ComponentConnection
	ConnectionsTo       map[string][]*ComponentConnection
}

func Analyze(settings *AnalysisSettings) (*Results, error) {
	allExtensions := defaultExtensions()
	for _, extension := range settings.Extensions {
		allExtensions = append(allExtensions, extension)
	}
	initializables := getExtensions[Initializable](allExtensions)

	for _, initializable := range initializables {
		initializable.Init(settings)
	}

	var allSnippets []*Snippet
	lock := sync.Mutex{}

	providers := getExtensions[SnippetProvider](allExtensions)
	walker.WalkDirectoryConcurrently(settings.RootPath, func(file walker.OpenedFile) {
		var foundSnippets []*Snippet
		for _, provider := range providers {
			foundSnippets = append(foundSnippets, provider.GetSnippetsFromFile(file)...)
		}
		lock.Lock()
		allSnippets = append(allSnippets, foundSnippets...)
		lock.Unlock()
	})
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

	snippetEditors := getExtensions[SnippetEditor](allExtensions)
	for _, editor := range snippetEditors {
		for _, snippet := range allSnippets {
			editor.EditSnippet(snippet)
		}
	}
	results := CalculateResults(settings.RootPath, allSnippets)

	resultEditors := getExtensions[ResultEditor](allExtensions)

	for _, editor := range resultEditors {
		editor.EditResults(results)
	}
	return results, nil
}

func defaultExtensions() []Extension {
	return []Extension{
		&rootPathStripper{},
	}
}

func CalculateResults(rootPath string, allSnippets []*Snippet) *Results {
	//set Directory name for each Snippet
	setDirectories(allSnippets)
	setComponents(allSnippets)

	//group Snippets by Directory
	allGroups := MultiGroupSnippetsBy(allSnippets, map[string]GroupSnippetByFunc{
		"ByDirectory": ByDirectory,
		"ByComponent": ByComponent,
		"ByFile":      ByFile,
		"ByType":      ByType,
	})

	byComponent := allGroups["ByComponent"]
	byType := allGroups["ByType"]

	connections := getConnections(byType, byComponent)

	componentConnectionsByFrom := GroupConnectionsBy(connections, func(connection *ComponentConnection) string {
		return connection.From
	})
	componentConnectionsByTo := GroupConnectionsBy(connections, func(connection *ComponentConnection) string {
		return connection.To
	})
	return &Results{
		RootDirectory:       rootPath,
		Snippets:            allSnippets,
		SnippetsByDirectory: allGroups["ByDirectory"],
		SnippetsByComponent: allGroups["ByComponent"],
		SnippetsByFile:      allGroups["ByFile"],
		SnippetsByType:      allGroups["ByType"],
		Connections:         connections,
		ConnectionsFrom:     componentConnectionsByFrom,
		ConnectionsTo:       componentConnectionsByTo,
	}
}

func setDirectories(s []*Snippet) {
	for _, snippet := range s {
		fileName := snippet.File
		snippet.Directory = fileName[:strings.LastIndex(fileName, "/")]
	}
}

func setComponents(s []*Snippet) {
	componentDeclarations := FilterSnippets(s, func(snippet *Snippet) bool {
		return snippet.Type == ComponentDeclaration
	})
	snippetsByFile := GroupSnippetsBy(s, ByFile)
	componentDeclarationsByFile := GroupSnippetsBy(componentDeclarations, ByFile)

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

func getConnections(snippetsByType map[string][]*Snippet, snippetsByComponent map[string][]*Snippet) []*ComponentConnection {
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
