package walker

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeTree(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func paths(t *testing.T, root string, opts ...Options) []string {
	t.Helper()
	files, err := GetAllFiles(root, opts...)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range files {
		out = append(out, f.Path())
	}
	sort.Strings(out)
	return out
}

func TestIgnorePatternsLayerOnTheTree(t *testing.T) {
	root := writeTree(t, "src/a.go", "src/gen/b.go", "docs/x.md", "docs/CODEOWNERS", "keep/c.go")
	got := paths(t, root, Options{IgnorePatterns: []string{"docs/", "src/gen/", "# a comment", ""}})
	// docs/ is ignored, except the ownership file, which never is.
	want := []string{"docs/CODEOWNERS", "keep/c.go", "src/a.go"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if len(paths(t, root)) != 5 {
		t.Errorf("without options every file is read")
	}
}

func TestMatcher(t *testing.T) {
	m := Matcher([]string{"vendor/", "*.min.js", "!keep.min.js"})
	for path, want := range map[string]bool{
		"vendor/lib/a.go":   true,
		"web/app.min.js":    true,
		"web/app.js":        false,
		"CODEOWNERS":        false,
		"vendor/CODEOWNERS": false,
	} {
		if got := m(path); got != want {
			t.Errorf("%s: got %v, want %v", path, got, want)
		}
	}
	if Matcher(nil)("anything") {
		t.Error("no patterns match nothing")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
