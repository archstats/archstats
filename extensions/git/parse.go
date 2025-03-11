package git

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"os"
	"os/exec"
	filepath "path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rawCommit struct {
	Repo        string
	Hash        string
	Time        time.Time
	AuthorName  string
	AuthorEmail string
	Message     string
	Files       []*rawPartOfCommit
}

type rawPartOfCommit struct {
	Repo      string
	Additions int
	Deletions int
	Path      string
}

func getGitCommitsFromAllReposConcurrently(root string, gitRepos []string) ([]*rawCommit, error) {
	log.Info().Msgf("Found %d git repositories", len(gitRepos))

	waitGroup := sync.WaitGroup{}
	lock := sync.RWMutex{}
	commitsByRepo := map[string][]*rawCommit{}
	errorsByRepo := map[string]error{}
	waitGroup.Add(len(gitRepos))

	for _, repo := range gitRepos {
		go func(repo string) {
			log.Info().Msgf("Parsing git log for %s", repo)
			commits, err := parseGitLog(root + "/" + repo)
			lock.Lock()

			if err == nil {
				commitsByRepo[repo] = commits
			} else {
				errorsByRepo[repo] = err
			}

			lock.Unlock()
			waitGroup.Done()
			log.Info().Msgf("Parsed %d commits for %s", len(commits), repo)
		}(repo)
	}

	waitGroup.Wait()

	if len(errorsByRepo) > 0 {
		errorStrings := []string{}
		for repo, err := range errorsByRepo {
			errorStrings = append(errorStrings, fmt.Sprintf("%s: %s", repo, err))
		}
		return nil, fmt.Errorf("failed to parse git logs: %s", strings.Join(errorStrings, ", "))
	}

	var commits []*rawCommit
	for _, repoCommits := range commitsByRepo {
		commits = append(commits, repoCommits...)
	}
	return commits, nil
}

// findGitRepos recursively searches for directories containing .git repositories.
func findGitRepos(root string) ([]string, error) {
	var gitRepos []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			gitPath := filepath.Join(path, ".git")
			_, err := os.Stat(gitPath)
			if err == nil {
				gitRepos = append(gitRepos, getDir(trimRepoPath(root, gitPath)))
				// Skip further traversal within this .git directory
				return filepath.SkipDir
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return gitRepos, nil
}

func parseGitLog(path string) ([]*rawCommit, error) {
	// Check if the Git command exists
	if !gitCommandExists() {
		return nil, fmt.Errorf("git command not found")
	}

	// Run 'git log' command
	cmd := exec.Command("git",
		"-C", filepath.Clean(path),
		"log", "--all", "--numstat", "--no-renames", "--pretty=format:[-archstatscommit-]%h--%at--%an--%ae--%s--")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run git log command: %s", err)
	}
	outputString := string(output)
	return parseGitLogString(path, outputString), nil
}

func parseGitLogString(repo, outputString string) []*rawCommit {

	// Split the output into individual commits
	commitStrings := strings.Split(outputString, "[-archstatscommit-]")

	return lo.Map(commitStrings[1:], func(commitRaw string, _ int) *rawCommit {
		return parseCommitString(repo, commitRaw)
	})
}

func parseCommitString(repo, commitRaw string) *rawCommit {
	// Parse commit data
	commitStrings := strings.Split(commitRaw, "--")
	// Let's pray this doesn't  fail ;)
	unixTimestamp, _ := strconv.ParseInt(commitStrings[1], 10, 64)

	commit := &rawCommit{
		Repo:        repo,
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
				Repo:      repo,
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
