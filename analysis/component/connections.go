package component

import (
	"github.com/RyanSusana/archstats/analysis/file"
)

// Connection is a connection between two components.
type Connection struct {
	From string
	To   string
	//The file in which the connection is made. The from side.
	File  string
	Begin *file.Position
	End   *file.Position
}

func GetConnections(snippetsByType file.SnippetGroup, snippetsByComponent file.SnippetGroup) []*Connection {
	var toReturn []*Connection
	from := snippetsByType[file.ComponentImport]
	for _, snippet := range from {
		if _, componentExistsInCodebase := snippetsByComponent[snippet.Value]; componentExistsInCodebase {
			toReturn = append(toReturn, &Connection{
				From:  snippet.Component,
				To:    snippet.Value,
				File:  snippet.File,
				Begin: snippet.Begin,
				End:   snippet.End,
			})
		}
	}
	return toReturn
}
