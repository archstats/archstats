package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) commitViewFactory(*core.Results) *core.View {
	rows := partsOfCommitToRows(e.commitParts)
	return &core.View{
		Name: "git_commits",
		Columns: []*core.Column{
			core.StringColumn(File),
			core.StringColumn(Component),
			core.StringColumn("repository"),
			core.StringColumn(CommitHash),
			core.DateColumn(CommitTime),
			core.StringColumn(AuthorName),
			core.StringColumn(AuthorEmail),
			core.StringColumn(CommitMessage),
			core.IntColumn(CommitFileAdditions),
			core.IntColumn(CommitFileDeletions),
		},
		Rows: rows,
	}
}
func partsOfCommitToRows(parts []*commits.PartOfCommit) []*core.Row {
	return lo.Map(parts, func(part *commits.PartOfCommit, _ int) *core.Row {
		return &core.Row{
			Data: map[string]interface{}{
				File:                part.File,
				Component:           part.Component,
				CommitHash:          part.Commit,
				"repository":        part.Repo,
				CommitTime:          part.Time,
				AuthorName:          part.Author,
				AuthorEmail:         part.AuthorEmail,
				CommitMessage:       part.Message,
				CommitFileAdditions: part.Additions,
				CommitFileDeletions: part.Deletions,
			},
		}
	})
}
