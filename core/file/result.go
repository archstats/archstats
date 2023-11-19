package file

type StatRecord struct {
	StatType string
	Value    interface{}
}

type Results struct {
	Directory string
	Component string
	Name      string
	Stats     []*StatRecord
	Snippets  []*Snippet
}
