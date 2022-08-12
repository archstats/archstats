package snippets

type AnalysisSettings struct {
	RootPath   string
	Extensions []Extension
}

type Extension interface{}

type Initializable interface {
	Init(settings *AnalysisSettings)
}

type SnippetProvider interface {
	GetSnippetsFromFile(File) []*Snippet
}

type SnippetEditor interface {
	EditSnippet(*Snippet)
}

type ResultEditor interface {
	EditResults(results *Results)
}

func getExtensions[K Extension](extensions []Extension) []K {
	var toReturn []K
	for _, extension := range extensions {

		editor, isEditor := extension.(K)
		if isEditor {
			toReturn = append(toReturn, editor)
		}
	}
	return toReturn
}
