package commits

import "time"

type PartOfCommit struct {
	Component   string
	Repo        string
	Commit      string
	Time        time.Time
	File        string
	Directory   string
	Author      string
	AuthorEmail string
	Message     string
	Additions   int
	Deletions   int
	// PathAtCommit is the file's name in this commit; File is its name now.
	PathAtCommit string
	// ChangeKind is "modify" or "rename".
	ChangeKind string
	// PureRename is a move that changed no lines: kept in the commit table
	// as evidence, left out of commit counts and co-change.
	PureRename bool
}

type CommitHashes []string
