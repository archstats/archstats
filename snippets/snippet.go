package snippets

import (
	"os"
)

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

type SnippetProvider interface {
	GetSnippetsFromFile(File) []*Snippet
}
