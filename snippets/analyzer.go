package snippets

import (
	"strings"
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

type Extension interface{}

type AnalysisSettings struct {
	SnippetProviders []SnippetProvider
}

func CalculateResults(allSnippets []*Snippet) *Results {
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
			})
		}
	}
	return toReturn
}
