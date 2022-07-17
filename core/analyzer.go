package core

import (
	"strings"
)

const (
	ComponentDeclaration = "componentDeclaration"
	ComponentImport      = "componentImport"
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

type SnippetGroup map[string][]*Snippet
type groupSnippetByFunc func(*Snippet) string

type ComponentConnection struct {
	From string
	To   string
}
type Extension interface{}

type AnalysisSettings struct {
	SnippetProvider []SnippetProvider
}

func Analyze(rootPath string, settings AnalysisSettings) (*Results, error) {

	snippets := getSnippetsFromDirectory(rootPath, settings.SnippetProvider)

	return calculateResults(rootPath, snippets), nil
}

func calculateResults(root string, snippets []*Snippet) *Results {
	//set Directory name for each Snippet
	setDirectories(snippets)
	setComponents(snippets)

	//group Snippets by Directory
	allGroups := MultiGroupSnippetsBy(snippets, map[string]groupSnippetByFunc{
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
		RootDirectory:       root,
		Snippets:            snippets,
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
