package git

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"io/fs"
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

func (e *extension) getGitCommitsFromAllReposConcurrently(root string, gitRepos []string) ([]*rawCommit, error) {
	log.Info().Msgf("Found %d git repositories", len(gitRepos))

	waitGroup := sync.WaitGroup{}
	lock := sync.RWMutex{}
	commitsByRepo := map[string][]*rawCommit{}
	errorsByRepo := map[string]error{}
	waitGroup.Add(len(gitRepos))

	for _, repo := range gitRepos {
		go func(repo string) {
			log.Info().Msgf("Parsing git log for %s", repo)
			commits, err := e.parseGitLog(root + "/" + repo)
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

// repoScan is what a walk of a directory tree found: the git repositories
// under it, and the directories it could not read.
type repoScan struct {
	repos      []string
	unreadable []string
}

// findGitRepos lists every git repository under root, as a path relative to
// it: "" when the root is itself a repository, "repos/alpha" for a checkout
// inside a directory of them.
//
// A repository's own contents are never descended into, so a workspace of
// checkouts costs a step per checkout rather than a walk of everything in
// them.
//
// A directory that cannot be read is skipped and named, rather than ending
// the walk. It used to end it: one unreadable directory anywhere under the
// root returned no repositories at all, and the caller logged the error and
// scanned on with no git history whatsoever.
func findGitRepos(root string) repoScan {
	var scan repoScan

	// The error is the callback's to handle; it never reaches here, because
	// the callback returns nil for every directory it cannot read.
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			scan.unreadable = append(scan.unreadable, path)
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
			return nil
		}
		// filepath.Rel rather than trimming the root off by hand, and
		// ToSlash because everything downstream -- component names, the
		// views, the UI -- speaks in forward slashes. Trimming by hand went
		// through getDir, which asks whether the path contains "/" and
		// answers "" when it does not: on Windows filepath.Join builds
		// backslashes, so every repository in a workspace was reported as
		// the root and they all collapsed into one.
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		scan.repos = append(scan.repos, rel)
		// Nothing inside a repository is another repository worth reporting.
		return filepath.SkipDir
	})

	return scan
}

// HasRepos reports whether there is a git repository under root: the root
// itself, or any checkout inside it.
//
// Extension discovery asks this rather than stat-ing root/.git, which only
// ever saw a root that was itself a repository. A workspace holding several
// checkouts -- the layout this extension has always read, one git log per
// repository -- has no .git of its own, so the extension was never switched
// on and the scan carried no history, no authors and no logical coupling.
func HasRepos(root string) bool {
	return len(findGitRepos(root).repos) > 0
}

func (e *extension) parseGitLog(path string) ([]*rawCommit, error) {
	// Check if the Git command exists
	if !gitCommandExists() {
		return nil, fmt.Errorf("git command not found")
	}

	argsRaw := []string{
		"-C", filepath.Clean(path),
	}
	if e.GitSince != "" {
		argsRaw = append(argsRaw, "--since", e.GitSince)
	}
	if e.GitAfter != "" {
		argsRaw = append(argsRaw, "--after", e.GitAfter)
	}
	argsRaw = append(argsRaw, "log", "--all", "--numstat", "--no-renames", "--pretty=format:[-archstatscommit-]%h--%at--%an--%ae--%s--")

	cmd := exec.Command(
		"git",
		argsRaw...,
	)
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

// LazyRepo is a clone that does not hold the objects a scan needs.
type LazyRepo struct {
	Dir string
	// The filter it was cloned with, e.g. "blob:none".
	Filter string
	// The config setting to unset to make it whole again.
	Setting string
}

// Advice is the one command that makes a scan of this repository fast.
func (l LazyRepo) Advice() string {
	return fmt.Sprintf("git -C %s config --unset %s && git -C %s fetch --refetch", l.Dir, l.Setting, l.Dir)
}

// lazyRepos finds clones that will make git go to the network mid-scan.
//
// `git log --numstat` has to diff every commit, and a partial clone
// (`--filter=blob:none`, what most "fast clone" advice now recommends) does
// not have the blobs to diff with. Git fetches them from the remote as it
// walks, so the scan becomes thousands of HTTPS round trips: on
// sakaiproject/sakai, 66,375 commits ran for over twelve minutes having spent
// ten seconds of that computing. Nothing in the output said so, which is the
// part worth fixing -- the wait belongs to the remote, and one command ends
// it.
// partialCloneFilter reads the filter a repository was cloned with, if any.
func partialCloneFilter(dir string) (setting string, filter string, ok bool) {
	out, err := exec.Command("git", "-C", dir, "config", "--get-regexp", `remote\..*\.partialclonefilter`).Output()
	if err != nil || len(out) == 0 {
		return "", "", false
	}
	// "remote.origin.partialclonefilter blob:none" -- the setting has to be
	// named back exactly, because `git fetch --refetch` on its own honours
	// the filter it finds and fetches nothing new. Measured: it returned in
	// seven seconds and left the clone just as partial.
	setting, filter, _ = strings.Cut(strings.TrimSpace(string(out)), " ")
	return setting, filter, setting != ""
}

func lazyRepos(rootPath string, repos []string) []LazyRepo {
	var found []LazyRepo
	for _, repo := range repos {
		// Repositories are named relative to the root, and the root itself is
		// named by the empty string.
		dir := filepath.Clean(filepath.Join(rootPath, repo))
		setting, filter, ok := partialCloneFilter(dir)
		if !ok || !missesObjects(dir) {
			continue
		}
		found = append(found, LazyRepo{Dir: dir, Filter: filter, Setting: setting})
	}
	return found
}

func warnAboutLazyRepos(rootPath string, repos []string) {
	for _, lazy := range lazyRepos(rootPath, repos) {
		log.Warn().Msgf(
			"%s is a partial clone (%s). Reading its history needs objects it does not have, so git fetches them from the remote while scanning and the scan will be slow. To download them once: `%s`.",
			lazy.Dir, lazy.Filter, lazy.Advice(),
		)
	}
}

// missesObjects asks whether this clone would actually have to go to the
// network, rather than assuming it from the config.
//
// The filter setting survives the fix -- `git fetch --refetch` re-adds it,
// and a repository that now holds every object still reads as a partial
// clone -- so warning on the setting alone cries wolf at a healthy
// repository. Instead the oldest commit, the one most likely to be missing
// its blobs, is diffed with lazy fetching switched off: if that works,
// nothing here needs saying. Two cheap commands, no walk of the whole
// history.
func missesObjects(dir string) bool {
	oldest, err := exec.Command("git", "-C", dir, "rev-list", "--all", "--max-parents=0", "-n", "1").Output()
	if err != nil {
		return false
	}
	commit := strings.TrimSpace(string(oldest))
	if commit == "" {
		return false
	}
	probe := exec.Command("git", "-C", dir, "log", "--numstat", "--no-renames", "-n", "1", "--format=", commit)
	probe.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	return probe.Run() != nil
}
