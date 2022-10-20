package analysis

type FileAnalyzer interface {
	AnalyzeFile(File) *FileResults
}

type FileResults struct {
	Name     string
	Stats    *Stats
	Snippets []*Snippet
}

func MergeFileResults(results []*FileResults) *FileResults {
	newResults := &FileResults{}
	for _, otherResult := range results {
		newResults.Stats = MergeMultipleStats([]*Stats{newResults.Stats, otherResult.Stats})
		newResults.Snippets = append(newResults.Snippets, otherResult.Snippets...)
	}
	return newResults
}
