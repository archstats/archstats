package snippets

type SnippetGroup map[string][]*Snippet
type GroupSnippetByFunc func(*Snippet) string

//filter Snippets by function
func FilterSnippets(snippets []*Snippet, filter func(*Snippet) bool) []*Snippet {
	toReturn := make([]*Snippet, 0)
	for _, snippet := range snippets {
		if filter(snippet) {
			toReturn = append(toReturn, snippet)
		}
	}
	return toReturn
}

func GroupSnippetsBy(snippets []*Snippet, groupBy GroupSnippetByFunc) SnippetGroup {
	return MultiGroupSnippetsBy(snippets, map[string]GroupSnippetByFunc{
		"": groupBy,
	})[""]
}

func MultiGroupSnippetsBy(snippets []*Snippet, groupBys map[string]GroupSnippetByFunc) map[string]SnippetGroup {
	toReturn := make(map[string]SnippetGroup)
	for s, _ := range groupBys {
		toReturn[s] = make(map[string][]*Snippet)
	}
	for _, snippet := range snippets {
		for name, groupBy := range groupBys {
			group := groupBy(snippet)
			toReturn[name][group] = append(toReturn[name][group], snippet)
		}
	}
	return toReturn
}

func ByFile(snippet *Snippet) string {
	return snippet.File
}
func ByType(s *Snippet) string {
	return s.Type
}
func ByDirectory(snippet *Snippet) string {
	return snippet.Directory
}
func ByComponent(snippet *Snippet) string {
	return snippet.Component
}
