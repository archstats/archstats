package git

import (
	"path"
	"sort"
	"strings"
)

// splitRename reads a numstat path that records a rename, in either of the
// two forms git writes: "old => new", or with the common parts factored out,
// "src/{old => new}/file.go". It reports ok=false for an ordinary path.
func splitRename(p string) (from, to string, ok bool) {
	if open := strings.Index(p, "{"); open >= 0 {
		if close := strings.Index(p[open:], "}"); close > 0 {
			close += open
			inner := p[open+1 : close]
			if a, b, found := strings.Cut(inner, " => "); found {
				prefix, suffix := p[:open], p[close+1:]
				return cleanJoin(prefix + a + suffix), cleanJoin(prefix + b + suffix), true
			}
		}
	}
	if a, b, found := strings.Cut(p, " => "); found {
		return a, b, true
	}
	return "", "", false
}

// cleanJoin removes the empty segment a factored rename leaves behind:
// "a/{ => sub}/f.go" reads "a//f.go" for its old side.
func cleanJoin(p string) string {
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return strings.TrimPrefix(p, "/")
}

// followRenames rewrites every commit's paths to the name each file has now,
// so a file's history survives being moved. Commits are walked newest to
// oldest per repository; a rename from A to B means every older mention of A
// is the file now called whatever B is called now. A path reused after a
// move stays itself: the alias only applies to commits older than the move.
func followRenames(commits []*rawCommit) {
	byRepo := map[string][]*rawCommit{}
	for _, c := range commits {
		byRepo[c.Repo] = append(byRepo[c.Repo], c)
	}
	for _, list := range byRepo {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Time.After(list[j].Time) })
		alias := map[string]string{}
		resolve := func(p string) string {
			seen := 0
			for {
				next, ok := alias[p]
				if !ok || next == p || seen > 64 {
					return p
				}
				p = next
				seen++
			}
		}
		for _, c := range list {
			var renames [][2]string
			for _, f := range c.Files {
				f.PathAtCommit = f.Path
				if f.OldPath != "" {
					f.ChangeKind = ChangeRename
					renames = append(renames, [2]string{f.OldPath, f.Path})
				} else if f.ChangeKind == "" {
					f.ChangeKind = ChangeModify
				}
				f.Path = resolve(f.Path)
			}
			// Aliases take effect after the commit's own rows are named: the
			// rename commit itself already uses the new path.
			for _, r := range renames {
				alias[r[0]] = resolve(r[1])
			}
		}
	}
}

const (
	ChangeModify = "modify"
	ChangeRename = "rename"
)

// dirOfPath is path.Dir without "." for a root-level file.
func dirOfPath(p string) string {
	d := path.Dir(p)
	if d == "." {
		return ""
	}
	return d
}
