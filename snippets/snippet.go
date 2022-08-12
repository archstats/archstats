package snippets

import (
	"os"
)

type OtherProperties map[string]interface{}
type Snippet struct {
	File      string `json:"file"`
	Directory string `json:"directory"`
	Component string `json:"component"`
	Type      string `json:"type"`
	Begin     int    `json:"begin"`
	End       int    `json:"end"`
	Value     string `json:"value"`
	OtherProperties
}
type FileDescription interface {
	Path() string
	Info() os.FileInfo
}
type File interface {
	FileDescription
	Content() []byte
}
