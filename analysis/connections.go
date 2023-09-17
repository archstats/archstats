package analysis

import (
	"github.com/RyanSusana/archstats/analysis/file"
)

// ComponentConnection is a connection between two components.
type ComponentConnection struct {
	From string
	To   string
	//The file in which the connection is made. The from side.
	File string
}

func getConnections(snippetsByType file.SnippetGroup, snippetsByComponent file.SnippetGroup) []*ComponentConnection {
	var toReturn []*ComponentConnection
	from := snippetsByType[file.ComponentImport]
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
