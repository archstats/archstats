package git

import (
	"sort"
	"strings"
)

// canonicalizeAuthors gives every commit by the same person the same name.
//
// Git records whatever name the committer's machine was configured with, so
// one person routinely appears as "Jeff Fischer" and "jefffischer", and the
// author list counted them twice -- a contributor's top collaborator was
// themself. `%aN`/`%aE` already applied the project's .mailmap; this handles
// the projects without one.
//
// Two commits are the same person when they share an email, when they share
// a full name (one containing a space, compared without regard to case)
// under different emails, or when a GitHub no-reply login is that full name
// with the spaces taken out. A bare
// handle such as "alex" is never enough on its own, and placeholder emails
// ("none@none", "root@localhost") never join anyone. The name kept is the
// full name the person committed under most often, or their most used handle
// if they never gave one.
func canonicalizeAuthors(all []*rawCommit) {
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if p, ok := parent[x]; ok && p != x {
			r := find(p)
			parent[x] = r
			return r
		}
		parent[x] = x
		return x
	}
	union := func(a, b string) { parent[find(a)] = find(b) }

	for _, c := range all {
		// " Voronkov Vitalii" was listed as its own author, leading space
		// and all.
		c.AuthorName = strings.Join(strings.Fields(c.AuthorName), " ")
	}
	nameKey := func(name string) string { return "n:" + strings.ToLower(name) }
	keyOf := func(c *rawCommit) string {
		if isPlaceholderEmail(c.AuthorEmail) {
			return nameKey(c.AuthorName)
		}
		return "e:" + strings.ToLower(strings.TrimSpace(c.AuthorEmail))
	}
	// A full name written without its spaces, for GitHub's no-reply
	// addresses: "StanislavFedorov" committing through the web UI is the
	// "Stanislav Fedorov" of the command line.
	squashed := map[string]string{}
	for _, c := range all {
		k := keyOf(c)
		find(k)
		if isFullName(c.AuthorName) {
			union(nameKey(c.AuthorName), k)
			squashed[strings.ToLower(strings.ReplaceAll(c.AuthorName, " ", ""))] = nameKey(c.AuthorName)
		}
	}
	for _, c := range all {
		if login := gitHubLogin(c.AuthorEmail); login != "" {
			if full, ok := squashed[login]; ok {
				union(keyOf(c), full)
			}
		}
	}

	// The most used name within each identity, ties broken alphabetically so
	// a rescan names people the same way.
	counts := map[string]map[string]int{}
	for _, c := range all {
		root := find(keyOf(c))
		if counts[root] == nil {
			counts[root] = map[string]int{}
		}
		counts[root][c.AuthorName]++
	}
	chosen := map[string]string{}
	for root, names := range counts {
		best := make([]string, 0, len(names))
		for n := range names {
			best = append(best, n)
		}
		sort.Slice(best, func(i, j int) bool {
			// A full name over a handle: "Marie Standeven" rather than the
			// "marieStandeven" of her GitHub commits, which outnumber them.
			if fi, fj := isFullName(best[i]), isFullName(best[j]); fi != fj {
				return fi
			}
			if names[best[i]] != names[best[j]] {
				return names[best[i]] > names[best[j]]
			}
			return best[i] < best[j]
		})
		chosen[root] = best[0]
	}
	for _, c := range all {
		c.AuthorName = chosen[find(keyOf(c))]
	}
}

// gitHubLogin is the login in a GitHub no-reply address
// ("42337700+StanislavFedorov@users.noreply.github.com"), lowercased, or "".
func gitHubLogin(email string) string {
	e := strings.ToLower(strings.TrimSpace(email))
	local, ok := strings.CutSuffix(e, "@users.noreply.github.com")
	if !ok {
		return ""
	}
	if _, after, found := strings.Cut(local, "+"); found {
		local = after
	}
	return local
}

func isFullName(name string) bool {
	return strings.Contains(strings.TrimSpace(name), " ")
}

func isPlaceholderEmail(email string) bool {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" || !strings.Contains(e, "@") {
		return true
	}
	local, domain, _ := strings.Cut(e, "@")
	switch domain {
	case "none", "(none)", "localhost", "localhost.localdomain", "example.com", "example.org":
		return true
	}
	switch local {
	case "none", "nobody", "noreply", "no-reply", "unknown":
		return true
	}
	return false
}
