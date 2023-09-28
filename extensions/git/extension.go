package git

import (
	"github.com/RyanSusana/archstats/core"
	"github.com/RyanSusana/archstats/core/file"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/samber/lo"
	"strings"
	"time"
)

// TODO
// Per file / Per component:
// Code age in days
//
// Per file and author combination:
// Number of additions
// Number of deletions
// Number of commits

type Extension struct {
	repository *git.Repository
	// Represents an individual change in a commit. A commit can have multiple parts if it changes multiple files.
	commitParts []*partOfCommit
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterResultsEditor(e)
	settings.RegisterView(&core.ViewFactory{
		Name:           "git",
		CreateViewFunc: e.gitViewFactory,
	})

	var err error
	e.repository, err = git.PlainOpen(settings.RootPath())
	if err != nil {
		return err
	}

	e.commitParts, err = repositoryToCommitParts(e.repository)
	if err != nil {
		return err
	}
	return nil
}

func (e *Extension) EditResults(results *core.Results) {
	for _, part := range e.commitParts {
		part.component = results.FileToComponent[part.file]
	}
	setStats(results.StatsByFile, lo.GroupBy(e.commitParts, func(part *partOfCommit) string {
		return part.file
	}))
	setStats(results.StatsByComponent, lo.GroupBy(e.commitParts, func(part *partOfCommit) string {
		return part.component
	}))
}

func (e *Extension) gitViewFactory(*core.Results) *core.View {
	return &core.View{
		Name: "git",
		Columns: []*core.Column{
			core.StringColumn("component"),
			core.StringColumn("commit"),
			core.DateColumn("time"),
			core.StringColumn("file"),
			core.StringColumn("author"),
			core.StringColumn("author_email"),
			core.StringColumn("message"),
			core.IntColumn("additions"),
			core.IntColumn("deletions"),
		},
		Rows: commitPartToRows(e.commitParts),
	}
}

func commitPartToRows(parts []*partOfCommit) []*core.Row {
	return lo.Map(parts, func(part *partOfCommit, _ int) *core.Row {
		return &core.Row{
			Data: map[string]interface{}{
				"component":    part.component,
				"commit":       part.commit,
				"time":         part.time,
				"file":         part.file,
				"author":       part.author,
				"author_email": part.authorEmail,
				"message":      part.message,
				"additions":    part.additions,
				"deletions":    part.deletions,
			},
		}
	})
}

type partOfCommit struct {
	component   string
	commit      string
	time        time.Time
	file        string
	author      string
	authorEmail string
	message     string
	additions   int
	deletions   int
}

func setStats(statsByFile file.StatsGroup, perFile map[string][]*partOfCommit) {
	for filePath, commitParts := range perFile {
		statsByFile.SetStat(filePath, "additions_count", lo.SumBy(commitParts, func(part *partOfCommit) int {
			return part.additions
		}))
		statsByFile.SetStat(filePath, "deletions_count", lo.SumBy(commitParts, func(part *partOfCommit) int {
			return part.deletions
		}))
		statsByFile.SetStat(filePath, "commits_count", len(lo.UniqBy(commitParts, func(part *partOfCommit) string {
			return part.commit
		})))
		statsByFile.SetStat(filePath, "authors_count", len(lo.UniqBy(commitParts, func(part *partOfCommit) string {
			return part.author
		})))
	}
}

func repositoryToCommitParts(repository *git.Repository) ([]*partOfCommit, error) {
	var commitParts []*partOfCommit
	iter, _ := repository.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		PathFilter: func(s string) bool {
			return !strings.Contains(s, ".json")
		},
	})
	err := iter.ForEach(func(commit *object.Commit) error {
		stats, err := commit.Stats()
		if err != nil {
			return err
		}

		for _, stat := range stats {
			commitParts = append(commitParts, &partOfCommit{
				commit:      commit.Hash.String(),
				time:        commit.Author.When,
				file:        stat.Name,
				author:      commit.Author.Name,
				authorEmail: commit.Author.Email,
				message:     commit.Message,
				additions:   stat.Addition,
				deletions:   stat.Deletion,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return commitParts, nil
}
