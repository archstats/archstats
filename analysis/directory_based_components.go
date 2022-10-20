package analysis

import "strings"

//DirectoryBasedComponents is a FileResultsEditor that sets the component of a snippet to the directory the File is in.
type DirectoryBasedComponents struct{}

func (d *DirectoryBasedComponents) EditFileResults(snippet *Snippet) {
	snippet.Component = snippet.File[:strings.Index(snippet.File, "/")]
}
