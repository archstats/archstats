package file

type StatRecord struct {
	StatType string
	Value    interface{}
}

type Results struct {
	// TODO can I avoid exposing this field?
	Name     string
	Stats    []*StatRecord
	Snippets []*Snippet
}
