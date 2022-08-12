package snippets

import "strings"

type DirectoryBasedComponents struct{}

func (d DirectoryBasedComponents) EditSnippet(snippet *Snippet) {
	snippet.Component = snippet.File[:strings.Index(snippet.File, "/")]
}
