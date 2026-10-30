package engine

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The import rules ADR 0025 rests on. Core knows no extension; a language
// pack knows no domain extension; a domain extension knows its ecosystem
// plug-ins only through their contract, and they know it not at all. Only
// engine names everything.
func TestImportBoundaries(t *testing.T) {
	const mod = "github.com/archstats/archstats/"
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	domains := []string{"extensions/cicd", "extensions/navigation", "extensions/mobile", "extensions/rules", "extensions/components", "extensions/codesmells", "extensions/git"}
	rules := []struct {
		name string
		from func(pkg string) bool
		bans func(pkg, imp string) bool
	}{
		{"core imports no extension, engine or CLI",
			func(p string) bool { return strings.HasPrefix(p, "core") },
			func(_, imp string) bool {
				return strings.HasPrefix(imp, "extensions") || strings.HasPrefix(imp, "engine") || strings.HasPrefix(imp, "cmd")
			}},
		{"a language pack imports no domain extension",
			func(p string) bool { return strings.HasPrefix(p, "extensions/treesitter") },
			func(_, imp string) bool {
				for _, d := range domains {
					if imp == d || strings.HasPrefix(imp, d+"/") {
						return true
					}
				}
				return false
			}},
		{"a build tool plug-in imports the contract, not cicd",
			func(p string) bool { return strings.HasPrefix(p, "extensions/cicd/buildtool") },
			func(_, imp string) bool { return imp == "extensions/cicd" }},
		{"cicd imports no build tool plug-in",
			func(p string) bool { return p == "extensions/cicd" },
			func(_, imp string) bool { return strings.HasPrefix(imp, "extensions/cicd/buildtool/") }},
	}
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if n := d.Name(); n == "testdata" || n == "temp_testdata" || strings.HasPrefix(n, ".") && p != root {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		pkg, err := build.ImportDir(p, 0)
		if err != nil {
			return nil // no Go files here
		}
		for _, imp := range pkg.Imports {
			if !strings.HasPrefix(imp, mod) {
				continue
			}
			imp = strings.TrimPrefix(imp, mod)
			for _, r := range rules {
				if r.from(rel) && r.bans(rel, imp) {
					t.Errorf("%s: %s imports %s", r.name, rel, imp)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
