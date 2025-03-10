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
}

type CommitHashes []string
