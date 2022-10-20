package analysis

import (
	"os"
)

const (
	ComponentDeclaration = "component_declaration"
	ComponentImport      = "component_import"
	AbstractType         = "abstract_type"
	Type                 = "type"
	FileCount            = "file_count"
)

// A Snippet is a piece of text that is extracted from a file.
// Snippets are used to generate Stats for a code base.
// Snippets can have several types, for example "function" or "class".
type Snippet struct {
	File      string `json:"file"`
	Directory string `json:"directory"`
	Component string `json:"component"`
	Type      string `json:"type"`
	Begin     int    `json:"begin"`
	End       int    `json:"end"`
	Value     string `json:"value"`
}

type FileDescription interface {
	Path() string
	Info() os.FileInfo
}

type File interface {
	FileDescription
	Content() []byte
}
type SnippetGroup map[string][]*Snippet
type GroupSnippetByFunc func(*Snippet) string

func MultiGroupSnippetsBy(snippets []*Snippet, groupBys map[string]GroupSnippetByFunc) map[string]SnippetGroup {
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
