package git

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/samber/lo"
)

func (e *extension) commitViewFactory(*core.Results) *core.View {
	rows := partsOfCommitToRows(e.viewParts)
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
			core.StringColumn("path_at_commit"),
			core.StringColumn("change_kind"),
		},
		Rows: rows,
	}
}
func partsOfCommitToRows(parts []*commits.PartOfCommit) []*core.Row {
	return lo.Map(parts, func(part *commits.PartOfCommit, _ int) *core.Row {
		// A deleted file has no component. Stored as "", it matched every other
		// deleted file's "" in any query joining on component.
		var component interface{}
		if part.Component != "" {
			component = part.Component
		}
		return &core.Row{
			Data: map[string]interface{}{
				File:                part.File,
				Component:           component,
				CommitHash:          part.Commit,
				"repository":        part.Repo,
				CommitTime:          part.Time,
				AuthorName:          part.Author,
				AuthorEmail:         part.AuthorEmail,
				CommitMessage:       part.Message,
				CommitFileAdditions: part.Additions,
				CommitFileDeletions: part.Deletions,
				"path_at_commit":    part.PathAtCommit,
				"change_kind":       part.ChangeKind,
			},
		}
	})
}
