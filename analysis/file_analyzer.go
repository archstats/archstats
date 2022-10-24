package analysis

type StatRecord struct {
	StatType string
	Value    interface{}
}
type FileAnalyzer interface {
	AnalyzeFile(File) *FileResults
}

type FileResults struct {
	Name     string
	Stats    []*StatRecord
	Snippets []*Snippet
}

func mergeFileResults(results []*FileResults) *FileResults {
	newResults := &FileResults{}
	for _, otherResult := range results {
		newResults.Stats = append(newResults.Stats, otherResult.Stats...)
		newResults.Snippets = append(newResults.Snippets, otherResult.Snippets...)
	}
	return newResults
}
