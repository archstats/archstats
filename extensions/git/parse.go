package git

import (
	"fmt"
	"github.com/samber/lo"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type rawCommit struct {
	Hash        string
	Time        time.Time
	AuthorName  string
	AuthorEmail string
	Message     string
	Files       []*rawPartOfCommit
}

type rawPartOfCommit struct {
	Additions int
	Deletions int
	Path      string
}

func (e *extension) parseGitLog(path string) ([]*rawCommit, error) {
	// Check if the Git command exists
	if !gitCommandExists() {
		return nil, fmt.Errorf("git command not found")
	}

	// Run 'git log' command
	cmd := exec.Command("git",
		"-C", path,
		"log", "--all", "--numstat", "--no-renames", "--pretty=format:[-archstatscommit-]%h--%at--%an--%ae--%s--")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	outputString := string(output)
	return parseGitLogString(outputString), nil
}

func parseGitLogString(outputString string) []*rawCommit {

	// Split the output into individual commits
	commitStrings := strings.Split(outputString, "[-archstatscommit-]")

	return lo.Map(commitStrings[1:], func(commitRaw string, _ int) *rawCommit {
		return parseCommitString(commitRaw)
	})
}

func parseCommitString(commitRaw string) *rawCommit {
	// Parse commit data
	commitStrings := strings.Split(commitRaw, "--")
	// Let's pray this doesn't  fail ;)
	unixTimestamp, _ := strconv.ParseInt(commitStrings[1], 10, 64)

	commit := &rawCommit{
		Hash:        commitStrings[0],
		Time:        time.Unix(unixTimestamp, 0),
		AuthorName:  commitStrings[2],
		AuthorEmail: commitStrings[3],
		Message:     commitStrings[4],
	}

	// Parse file data
	fileStrings := strings.Split(strings.TrimSpace(commitStrings[5]), "\n")
	for _, fileString := range fileStrings {
		fields := strings.Fields(fileString)
		if len(fields) == 3 {
			additions := 0
			deletions := 0
			fmt.Sscanf(fields[0], "%d", &additions)
			fmt.Sscanf(fields[1], "%d", &deletions)
			commit.Files = append(commit.Files, &rawPartOfCommit{
				Additions: additions,
				Deletions: deletions,
				Path:      fields[2],
			})
		}
	}
	return commit
}

func gitCommandExists() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
