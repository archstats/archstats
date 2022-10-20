package snippets

//This SnippetProvider makes sure that all files have at least one snippet
type fileMarker struct {
}

func (f *fileMarker) GetSnippetsFromFile(file File) []*Snippet {
	return []*Snippet{
		{
			File: file.Path(),
			Type: "file_marker",
		},
	}
}
