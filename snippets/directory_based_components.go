package snippets

type DirectoryBasedComponents struct {
	Directory string
}

func (d *DirectoryBasedComponents) GetSnippetsFromFile(file File) []*Snippet {
	return nil
}
