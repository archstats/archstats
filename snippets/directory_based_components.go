package snippets

import "strings"

//DirectoryBasedComponents is a SnippetEditor that sets the component of a snippet to the directory the File is in.
type DirectoryBasedComponents struct{}

func (d *DirectoryBasedComponents) EditSnippet(snippet *Snippet) {
	snippet.Component = snippet.File[:strings.Index(snippet.File, "/")]
}
