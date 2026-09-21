package component

import (
	"github.com/archstats/archstats/core/file"
)

// The kinds of dependency an edge can be. An edge's kind decides whether it
// counts as coupling: a type-only import is a real dependency on a shape and
// no dependency at all once the program runs.
const (
	// A static import, and the only kind archstats recorded before kinds
	// existed. The zero value, so an edge nobody classified is an import.
	KindImport = ""
	// TypeScript's `import type`, erased at compile time.
	KindTypeOnly = "type_only"
	// Resolved from a string at runtime: Django's get_class, a dynamic
	// import() with a computed specifier, a class name built from config.
	KindDynamic = "dynamic"
	// Composition rather than reference: Go struct embedding, Kotlin
	// delegation. gin embeds RouterGroup and ResponseWriter, and no
	// `implements` query can see either.
	KindEmbed = "embed"
	// Declared in a manifest with no import anywhere: a .csproj
	// ProjectReference, a composer require, a gradle project() dependency.
	KindManifest = "manifest"
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

// Kind names this edge's kind for display. An unclassified edge is an
// ordinary static import, which is what every edge was before kinds existed.
func (c *Connection) Kind() string {
	if c.Type == KindImport {
		return "import"
	}
	return c.Type
}

// IsRuntime reports whether this dependency still exists once the program
// runs. Only runtime edges enter the component graph, so instability,
// abstractness and cycles are computed on what actually couples.
func (c *Connection) IsRuntime() bool {
	return c.Type != KindTypeOnly
}

// RuntimeOnly filters a list to the edges that survive compilation.
func RuntimeOnly(connections []*Connection) []*Connection {
	var out []*Connection
	for _, c := range connections {
		if c.IsRuntime() {
			out = append(out, c)
		}
	}
	return out
}

func (c *Connection) String() string {
	return c.From + " -> " + c.To + " in " + c.File + " [ " + c.Begin.String() + " - " + c.End.String() + " ]"
}

func GetConnectionsFromSnippetImports(snippetsByType file.SnippetGroup, snippetsByComponent file.SnippetGroup) []*Connection {
	var toReturn []*Connection
	var from []*file.Snippet
	kindOf := map[*file.Snippet]string{}
	for _, snippet := range snippetsByType[file.ComponentImport] {
		from = append(from, snippet)
		kindOf[snippet] = KindImport
	}
	for _, snippet := range snippetsByType[file.ComponentImportTypeOnly] {
		from = append(from, snippet)
		kindOf[snippet] = KindTypeOnly
	}
	for _, snippet := range snippetsByType[file.ComponentImportDynamic] {
		from = append(from, snippet)
		kindOf[snippet] = KindDynamic
	}
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
				Type:  kindOf[snippet],
				File:  snippet.File,
				Begin: snippet.Begin,
				End:   snippet.End,
			})
		}
	}
	return toReturn
}
