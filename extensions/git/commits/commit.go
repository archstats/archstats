package commits

import "time"

type PartOfCommit struct {
	Component   string
	Commit      string
	Time        time.Time
	File        string
	Author      string
	AuthorEmail string
	Message     string
	Additions   int
	Deletions   int
}

type CommitHashes []string
