package core

import "os"

type File interface {
	Path() string
	Info() os.FileInfo
	Content() []byte
}
type SnippetProvider interface {
	GetSnippetsFromFile(File) []*Snippet
}

type Snippet struct {
	File      string
	Directory string
	Component string
	Type      string
	Begin     int
	End       int
	Value     string
}

func ByFile(snippet *Snippet) string {
	return snippet.File
}
func ByType(s *Snippet) string {
	return s.Type
}
func ByDirectory(snippet *Snippet) string {
	return snippet.Directory
}
func ByComponent(snippet *Snippet) string {
	return snippet.Component
}

//filter Snippets by function
func FilterSnippets(snippets []*Snippet, filter func(*Snippet) bool) []*Snippet {
	toReturn := make([]*Snippet, 0)
	for _, snippet := range snippets {
		if filter(snippet) {
			toReturn = append(toReturn, snippet)
		}
	}
	return toReturn
}

func GroupConnectionsBy(connections []*ComponentConnection, groupBy func(connection *ComponentConnection) string) map[string][]*ComponentConnection {
	toReturn := make(map[string][]*ComponentConnection)
	for _, connection := range connections {
		group := groupBy(connection)
		toReturn[group] = append(toReturn[group], connection)
	}
	return toReturn
}

func GroupSnippetsBy(snippets []*Snippet, groupBy groupSnippetByFunc) SnippetGroup {
	return MultiGroupSnippetsBy(snippets, map[string]groupSnippetByFunc{
		"": groupBy,
	})[""]
}

func MultiGroupSnippetsBy(snippets []*Snippet, groupBys map[string]groupSnippetByFunc) map[string]SnippetGroup {
	toReturn := make(map[string]SnippetGroup)
	for s, _ := range groupBys {
		toReturn[s] = make(map[string][]*Snippet)
	}
	for _, snippet := range snippets {
		for name, groupBy := range groupBys {
			group := groupBy(snippet)
			toReturn[name][group] = append(toReturn[name][group], snippet)
		}
	}
	return toReturn
}
