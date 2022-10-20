package analysis

// A SnippetEditor is a function that edits a snippet to remove the unwanted parts of the absolute path
type rootPathStripper struct {
	root string
}

func (p *rootPathStripper) Init(settings *Settings) {
	p.root = settings.RootPath
}

func (p *rootPathStripper) EditSnippet(snippet *Snippet) {
	snippet.File = snippet.File[len(p.root):]
}
