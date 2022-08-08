package snippets

import (
	"errors"
	"github.com/RyanSusana/archstats/walker"
	"sort"
	"sync"
)

func Analyze(rootPath string, settings AnalysisSettings) (*Results, error) {

	var allSnippets []*Snippet
	lock := sync.Mutex{}

	walker.WalkDirectoryConcurrently(rootPath, func(file walker.OpenedFile) {
		var foundSnippets []*Snippet
		for _, provider := range settings.SnippetProviders {
			foundSnippets = append(foundSnippets, provider.GetSnippetsFromFile(file)...)
		}
		lock.Lock()
		allSnippets = append(allSnippets, foundSnippets...)
		lock.Unlock()
	})
	// Pre-sort the snippets to make sure they are in the same order every time.
	sort.Slice(allSnippets, func(i, j int) bool {
		if allSnippets[i].File != allSnippets[j].File {
			return allSnippets[i].File < allSnippets[j].File
		}
		if allSnippets[i].Begin != allSnippets[j].Begin {
			return allSnippets[i].Begin < allSnippets[j].Begin
		}
		return allSnippets[i].End < allSnippets[j].End
	})
	if len(allSnippets) == 0 {
		return nil, errors.New("could not find any snippets")
	}
	return CalculateResults(rootPath, allSnippets), nil
}
