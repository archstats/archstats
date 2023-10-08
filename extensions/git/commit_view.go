package git

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
)

func (e *extension) commitViewFactory(*core.Results) *core.View {
	rows := partsOfCommitToRows(e.commitParts)
	return &core.View{
		Name: "git_commits",
		Columns: []*core.Column{
			core.StringColumn(File),
			core.StringColumn(Component),
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
func partsOfCommitToRows(parts []*partOfCommit) []*core.Row {
	return lo.Map(parts, func(part *partOfCommit, _ int) *core.Row {
		return &core.Row{
			Data: map[string]interface{}{
				File:                part.file,
				Component:           part.component,
				CommitHash:          part.commit,
				CommitTime:          part.time,
				AuthorName:          part.author,
				AuthorEmail:         part.authorEmail,
				CommitMessage:       part.message,
				CommitFileAdditions: part.additions,
				CommitFileDeletions: part.deletions,
			},
		}
	})
}
