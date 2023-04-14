package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/samber/lo"
)

func ok() {
}

type Results struct {
	FileStats map[string]map[string]int
}
type RawResults struct {
	Commits         []*Commit
	AuthorToCommits map[string][]*Commit
	FilesToCommits  map[string][]*Commit
}
type Commit struct {
	author string
	files  map[string]*object.FileStat
}

func x(commits []*Commit) *RawResults {
	rawResults := &RawResults{
		Commits:         commits,
		AuthorToCommits: make(map[string][]*Commit),
		FilesToCommits:  make(map[string][]*Commit),
	}

	for _, commit := range commits {
		rawResults.AuthorToCommits[commit.author] = append(rawResults.AuthorToCommits[commit.author], commit)
		for file := range commit.files {
			rawResults.FilesToCommits[file] = append(rawResults.FilesToCommits[file], commit)
		}
	}
	return rawResults
}
func repositoryToCommits(repository *git.Repository) []*Commit {
	var commits []*Commit
	iter, _ := repository.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
	})

	iter.ForEach(func(commit *object.Commit) error {
		theCommit := &Commit{}
		stats, err := commit.Stats()
		if err != nil {
			return err
		}

		theCommit.author = commit.Author.Name
		theCommit.files = lo.Associate(stats, func(fileStat object.FileStat) (string, *object.FileStat) {
			return fileStat.Name, &fileStat
		})
		commits = append(commits, theCommit)
		return nil
	})
	return commits
}
