package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSplitRename(t *testing.T) {
	cases := []struct{ in, from, to string }{
		{"old.go => new.go", "old.go", "new.go"},
		{"src/{a => b}/f.go", "src/a/f.go", "src/b/f.go"},
		{"{old => new}/f.go", "old/f.go", "new/f.go"},
		{"a/{ => sub}/f.go", "a/f.go", "a/sub/f.go"},
		{"a/{sub => }/f.go", "a/sub/f.go", "a/f.go"},
	}
	for _, c := range cases {
		from, to, ok := splitRename(c.in)
		assert.True(t, ok, c.in)
		assert.Equal(t, c.from, from, c.in)
		assert.Equal(t, c.to, to, c.in)
	}
	_, _, ok := splitRename("plain/path.go")
	assert.False(t, ok)
}

func TestFollowRenames(t *testing.T) {
	at := func(day int) time.Time { return time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC) }
	c := func(day int, files ...*rawPartOfCommit) *rawCommit {
		return &rawCommit{Repo: "r", Time: at(day), Files: files}
	}
	edit := func(p string) *rawPartOfCommit { return &rawPartOfCommit{Path: p, Additions: 1} }
	move := func(from, to string) *rawPartOfCommit { return &rawPartOfCommit{Path: to, OldPath: from} }

	commits := []*rawCommit{
		c(1, edit("a.go")),         // written as a.go
		c(2, move("a.go", "b.go")), // moved to b.go
		c(3, edit("b.go")),         // edited as b.go
		c(4, move("b.go", "c.go")), // moved again
		c(5, edit("a.go")),         // a new file that reuses the old name
		c(6, edit("c.go"), edit("a.go")),
	}
	followRenames(commits)
	names := func(day int) []string {
		var out []string
		for _, cm := range commits {
			if cm.Time.Equal(at(day)) {
				for _, f := range cm.Files {
					out = append(out, f.Path)
				}
			}
		}
		return out
	}
	assert.Equal(t, []string{"c.go"}, names(1), "a.go's history belongs to the file now called c.go")
	assert.Equal(t, []string{"c.go"}, names(2))
	assert.Equal(t, []string{"c.go"}, names(3))
	assert.Equal(t, []string{"a.go"}, names(5), "the reused name is its own file")
	assert.Equal(t, []string{"c.go", "a.go"}, names(6))
	assert.Equal(t, "a.go", commits[0].Files[0].PathAtCommit)
	assert.Equal(t, ChangeRename, commits[1].Files[0].ChangeKind)
}
