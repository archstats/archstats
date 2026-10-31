package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The repository's documents point at each other and at the code. A link or a
// path that no longer resolves sends a reader, or an agent, somewhere that is
// gone, so the documents are checked like code: every relative link resolves,
// and every backticked path into the source tree exists.

var (
	mdLink    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	codePath  = regexp.MustCompile("`((?:cmd|core|engine|extensions|docs|e2eTest|scripts|version)/[^`\\s]*)`")
	trailings = regexp.MustCompile(`(:\d+(-\d+)?|[.,;:])$`)
)

func documents(t *testing.T) []string {
	t.Helper()
	docs := []string{"README.md", "ARCHITECTURE.md", "DESCRIPTION.md", "CHANGELOG.md", "CLAUDE.md"}
	err := filepath.WalkDir("docs", func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") {
			docs = append(docs, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return docs
}

func TestDocumentLinksResolve(t *testing.T) {
	for _, doc := range documents(t) {
		raw, err := os.ReadFile(doc)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, m := range mdLink.FindAllStringSubmatch(text, -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			if _, err := os.Stat(filepath.Join(filepath.Dir(doc), target)); err != nil {
				t.Errorf("%s links to %s, which does not exist", doc, m[1])
			}
		}
		// A decision record describes the code as it was when it was taken.
		if strings.HasPrefix(filepath.ToSlash(doc), "docs/adr/") {
			continue
		}
		for _, m := range codePath.FindAllStringSubmatch(text, -1) {
			path := trailings.ReplaceAllString(m[1], "")
			if strings.ContainsAny(path, "*<{") {
				continue
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("%s names %s, which does not exist", doc, m[1])
			}
		}
	}
}
