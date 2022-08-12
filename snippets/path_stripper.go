package snippets

type rootPathStripper struct {
	root string
}

func (p *rootPathStripper) Init(settings *AnalysisSettings) {
	p.root = settings.RootPath
}

func (p *rootPathStripper) EditSnippet(snippet *Snippet) {
	snippet.File = snippet.File[len(p.root):]
}
