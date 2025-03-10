package component

import (
	"github.com/archstats/archstats/core/file"
)

// Connection is a connection (coupling) between two components.
type Connection struct {
	From string
	To   string
	Type string
	//The file in which the connection is made. The from side.
	File  string
	Begin *file.Position
	End   *file.Position
}

func (c *Connection) String() string {
	return c.From + " -> " + c.To + " in " + c.File + " [ " + c.Begin.String() + " - " + c.End.String() + " ]"
}

func GetConnectionsFromSnippetImports(snippetsByType file.SnippetGroup, snippetsByComponent file.SnippetGroup) []*Connection {
	var toReturn []*Connection
	from := snippetsByType[file.ComponentImport]
	for _, snippet := range from {
		connectionTo := snippet.Value
		if _, componentExistsInCodebase := snippetsByComponent[connectionTo]; componentExistsInCodebase {
			connectionFrom := snippet.Component

			// Skip self-references
			if connectionFrom == connectionTo {
				continue
			}
			toReturn = append(toReturn, &Connection{
				From:  connectionFrom,
				To:    connectionTo,
				File:  snippet.File,
				Begin: snippet.Begin,
				End:   snippet.End,
			})
		}
	}
	return toReturn
}
