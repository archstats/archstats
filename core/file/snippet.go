package file

const (
	ComponentDeclaration = "modularity__component__declarations"
	ComponentImport      = "modularity__component__imports"
	// An import the compiler erases: TypeScript's `import type`. It names a
	// real dependency on a shape, but no dependency at runtime, and counting
	// it as coupling inflates every TypeScript graph -- 681 of LibreChat's
	// 5,100 imports, an eighth of them, are written this way.
	ComponentImportTypeOnly = "modularity__component__imports__type"
	// A dependency named by a string and resolved when the program runs.
	// django-oscar makes 746 get_class/get_model calls against 1,645 static
	// imports: roughly 31% of its edges, and not an accident -- it is how
	// every one of its apps is made overridable. Drawn without them, a third
	// of that codebase's graph is missing and nothing says so.
	ComponentImportDynamic = "modularity__component__imports__dynamic"
	AbstractType         = "modularity__types__abstract"
	Type                 = "modularity__types__total"
	FileCount            = "complexity__files"
)

// A Snippet is a piece of text that is extracted from a file.
// Snippets are used to generate Stats for a code base.
// Snippets can have several types, for example "function" or "class".
type Snippet struct {
	File      string    `json:"file"`
	Directory string    `json:"directory"`
	Component string    `json:"component"`
	Type      string    `json:"type"`
	Begin     *Position `json:"begin"`
	End       *Position `json:"end"`
	Value     string    `json:"Value"`
}

type SnippetGroup map[string][]*Snippet
type GroupSnippetByFunc func(*Snippet) string

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
