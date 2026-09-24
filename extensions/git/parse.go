package git

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"io/fs"
	"os"
	"os/exec"
	filepath "path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type rawCommit struct {
	Repo        string
	Hash        string
	Time        time.Time
	AuthorName  string
	AuthorEmail string
	Message     string
	Files       []*rawPartOfCommit
	// Read with --cc --raw and listing files that differ from every parent.
	combined bool
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

// The fields of one commit header, in the order the format string asks for.
// Unit and record separators rather than printable ones: a subject reading
// "fix -- again" split the old "--"-delimited header and dropped every file
// of that commit.
const (
	commitMarker = "\x1e[-archstatscommit-]"
	fieldSep     = "\x1f"
	headerEnd    = "\x1d"
	gitLogFormat = "%x1e[-archstatscommit-]%h%x1f%at%x1f%aN%x1f%aE%x1f%s%x1d"
)

func (e *extension) parseGitLog(path string) ([]*rawCommit, error) {
	// Check if the Git command exists
	if !gitCommandExists() {
		return nil, fmt.Errorf("git command not found")
	}

	// core.quotepath=off keeps non-ASCII paths as written instead of
	// C-quoting them, which no file in the scan would ever match.
	argsRaw := []string{"-C", filepath.Clean(path), "-c", "core.quotepath=off", "log"}
	// Only the history of what is checked out. `--all` read every branch
	// and ref: commits that never reached the analysed code, and the same
	// change counted once per branch it was cherry-picked onto.
	argsRaw = append(argsRaw, "HEAD")
	// --since and --after are options of `log`, not of git itself; placed
	// before the subcommand they made git refuse to run.
	if e.GitSince != "" {
		argsRaw = append(argsRaw, "--since", e.GitSince)
	}
	if e.GitAfter != "" {
		argsRaw = append(argsRaw, "--after", e.GitAfter)
	}
	// %aN and %aE honour .mailmap, so a project's own record of who is who
	// is used before any guessing.
	argsRaw = append(argsRaw, "--numstat", "--no-renames", "--pretty=format:"+gitLogFormat)

	commitArgs := append(append([]string{}, argsRaw...), "--no-merges")
	output, err := exec.Command("git", commitArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run git log command: %s", err)
	}
	commits := parseGitLogString(path, string(output))

	// A merge changes a file when it differs from every parent: what was
	// written while resolving it. Those are the only files a merge reports
	// here; everything else it brings in was counted in the branch commits
	// that made it. Merges used to report nothing, so a file only ever
	// changed in a merge had no history at all -- 7 of Sylius's files.
	//
	// --numstat on a merge diffs it against its first parent, which would
	// count a branch's work a second time; --cc --raw lists the files that
	// differ from every parent, and only those rows are kept.
	mergeArgs := append(append([]string{}, argsRaw...), "--merges", "--cc", "--raw")
	mergeOutput, err := exec.Command("git", mergeArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run git log for merges: %s", err)
	}
	for _, merge := range parseGitLogString(path, string(mergeOutput)) {
		// A clean merge lists no such files, and its numstat rows are all
		// branch work already counted.
		if merge.combined && len(merge.Files) > 0 {
			commits = append(commits, merge)
		}
	}
	return commits, nil
}

func parseGitLogString(repo, outputString string) []*rawCommit {
	var out []*rawCommit
	for _, raw := range strings.Split(outputString, commitMarker)[1:] {
		if c := parseCommitString(repo, raw); c != nil {
			out = append(out, c)
		}
	}
	return out
}

func parseCommitString(repo, commitRaw string) *rawCommit {
	header, body, found := strings.Cut(commitRaw, headerEnd)
	if !found {
		return nil
	}
	fields := strings.Split(header, fieldSep)
	if len(fields) < 5 {
		return nil
	}
	unixTimestamp, _ := strconv.ParseInt(fields[1], 10, 64)

	commit := &rawCommit{
		Repo:        repo,
		Hash:        fields[0],
		Time:        time.Unix(unixTimestamp, 0),
		AuthorName:  asUTF8(fields[2]),
		AuthorEmail: asUTF8(fields[3]),
		// A subject can itself contain the separator in principle; keep
		// whatever follows rather than truncating it.
		Message: asUTF8(strings.Join(fields[4:], fieldSep)),
	}

	// numstat rows are tab-separated. Splitting them on whitespace dropped
	// every file whose path contains a space.
	//
	// A merge read with --cc --raw also lists, as "::" rows, the files that
	// differ from every parent; when there are any, only those count.
	var combined map[string]bool
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if strings.HasPrefix(line, "::") {
			if _, path, ok := strings.Cut(strings.TrimRight(line, "\r"), "\t"); ok && path != "" {
				if combined == nil {
					combined = map[string]bool{}
				}
				combined[path] = true
			}
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if strings.HasPrefix(line, "::") {
			continue
		}
		parts := strings.SplitN(strings.TrimRight(line, "\r"), "\t", 3)
		if len(parts) != 3 || parts[2] == "" {
			continue
		}
		if combined != nil && !combined[parts[2]] {
			continue
		}
		additions, deletions := 0, 0
		// Binary files report "-" for both counts; they are changes all the same.
		fmt.Sscanf(parts[0], "%d", &additions)
		fmt.Sscanf(parts[1], "%d", &deletions)
		commit.Files = append(commit.Files, &rawPartOfCommit{
			Repo:      repo,
			Additions: additions,
			Deletions: deletions,
			Path:      parts[2],
		})
	}
	commit.combined = combined != nil
	return commit
}

// asUTF8 reads text git passed through undecoded. A commit made without an
// encoding header by a Latin-1 terminal is stored as Latin-1 and printed as
// such: Sylius has "Javier Gonz\xe1lez", which is not UTF-8 and broke every
// reader of the author table. Latin-1 is what git itself assumes of such
// commits, and every byte of it is a character.
func asUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); i++ {
		out = append(out, rune(s[i]))
	}
	return string(out)
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

// shallowRepos are the repositories cloned with --depth, whose history stops
// at a commit that is not the first. gin cloned that way reads one commit and
// one contributor, and a file's age is the age of the clone.
func shallowRepos(rootPath string, repos []string) map[string]bool {
	out := map[string]bool{}
	for _, repo := range repos {
		dir := filepath.Join(rootPath, repo)
		answer, err := exec.Command("git", "-C", dir, "rev-parse", "--is-shallow-repository").Output()
		if err == nil && strings.TrimSpace(string(answer)) == "true" {
			out[repo] = true
			log.Warn().Msgf("%s is a shallow clone, so its history is cut short: commit counts, contributors and file ages cover only what was fetched. `git -C %s fetch --unshallow` fetches the rest.", dir, dir)
		}
	}
	return out
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
