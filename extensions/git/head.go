package git

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// headInfo is what a repository's working tree was when it was scanned: the
// commit checked out, the branch, when that commit was made, and how many
// files differed from it. A snapshot is only a reproducible fact about code
// if it says which code.
type headInfo struct {
	Commit string
	Branch string
	Time   time.Time
	Dirty  int
	ok     bool
}

func readHead(dir string) headInfo {
	var h headInfo
	sha, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return h
	}
	h.Commit = strings.TrimSpace(string(sha))
	if branch, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		h.Branch = strings.TrimSpace(string(branch))
		if h.Branch == "HEAD" {
			h.Branch = "detached"
		}
	}
	if when, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%cI", "HEAD").Output(); err == nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(when))); err == nil {
			h.Time = t
		}
	}
	if status, err := exec.Command("git", "-C", dir, "status", "--porcelain", "--untracked-files=normal").Output(); err == nil {
		for _, line := range strings.Split(string(status), "\n") {
			if strings.TrimSpace(line) != "" {
				h.Dirty++
			}
		}
	}
	h.ok = true
	return h
}

func readHeads(root string, repos []string) map[string]headInfo {
	out := make(map[string]headInfo, len(repos))
	for _, repo := range repos {
		if h := readHead(filepath.Join(root, repo)); h.ok {
			out[repo] = h
		}
	}
	return out
}

// recordSnapshotInfo writes the repositories' identity into the scan's
// snapshot facts: the root repository's HEAD when the scan root is one, and
// in every case the newest commit time across repositories.
func (e *extension) recordSnapshotInfo(set func(key, value string)) {
	var newest time.Time
	for _, h := range e.heads {
		if h.Time.After(newest) {
			newest = h.Time
		}
	}
	if root, ok := e.heads[""]; ok {
		set("git_head_commit", root.Commit)
		set("git_branch", root.Branch)
		set("git_dirty_files", strconv.Itoa(root.Dirty))
	}
	if !newest.IsZero() {
		set("git_head_time", newest.UTC().Format(time.RFC3339))
	}
	set("git_max_changes_per_commit", strconv.Itoa(e.MaxChangesPerCommit))
	set("git_sweeping_commits", strconv.Itoa(e.sweepingTotal))
}
