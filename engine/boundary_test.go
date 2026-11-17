package engine

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
		{"a recogniser plug-in imports the contract, not navigation or another plug-in",
			func(p string) bool { return strings.HasPrefix(p, "extensions/navigation/recogniser/") },
			func(_, imp string) bool {
				return imp == "extensions/navigation" || strings.HasPrefix(imp, "extensions/navigation/recogniser/")
			}},
		{"the recogniser contract imports neither navigation nor a plug-in",
			func(p string) bool { return p == "extensions/navigation/recogniser" },
			func(_, imp string) bool { return strings.HasPrefix(imp, "extensions/") }},
		{"navigation imports no recogniser plug-in",
			func(p string) bool { return p == "extensions/navigation" },
			func(_, imp string) bool { return strings.HasPrefix(imp, "extensions/navigation/recogniser/") }},
		{"a domain extension imports no other domain extension",
			func(p string) bool { return domainOf(domains, p) != "" },
			func(p, imp string) bool {
				d := domainOf(domains, imp)
				return d != "" && d != domainOf(domains, p)
			}},
		{"only engine names a language: no extension outside treesitter imports a language pack",
			func(p string) bool {
				return strings.HasPrefix(p, "extensions/") && !strings.HasPrefix(p, "extensions/treesitter")
			},
			func(_, imp string) bool {
				return strings.HasPrefix(imp, "extensions/treesitter/") && imp != "extensions/treesitter/common"
			}},
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

// domainOf is the domain extension a package belongs to, or "".
func domainOf(domains []string, pkg string) string {
	for _, d := range domains {
		if pkg == d || strings.HasPrefix(pkg, d+"/") {
			return d
		}
	}
	return ""
}

// The other half of ADR 0025: core knows no ecosystem in its code either,
// not only in its imports. A string literal in core that names a language's
// file extension, a build tool's manifest, a package manager's folder or a
// framework is core branching on an ecosystem; that answer belongs to the
// language pack or the domain extension's plug-in. Comments may name them;
// tests may too.
//
// Rules that hold in every language (tests/, vendor/, data formats such as
// .json, .yaml, .xml, .md) are not ecosystems and are not matched.
func TestCoreNamesNoEcosystem(t *testing.T) {
	root, err := filepath.Abs("../core")
	if err != nil {
		t.Fatal(err)
	}
	ecosystem := []*regexp.Regexp{
		// A source file extension, alone or ending a name or glob.
		regexp.MustCompile(`(?i)(^|[*/\w])\.(java|kts?|scala|groovy|go|py|pyi|php|cs|csproj|vbproj|fsproj|fs|vb|tsx?|jsx?|mjs|cjs|vue|svelte|swift|dart|m|mm|rb|rs|cpp|cc|hpp|h|c|ex|exs|erl|clj)$`),
		// A manifest, lockfile or package manager folder.
		regexp.MustCompile(`(^|/)(pom\.xml|build\.gradle(\.kts)?|settings\.gradle(\.kts)?|package\.json|package-lock\.json|yarn\.lock|pnpm-lock\.yaml|tsconfig\.json|go\.mod|go\.sum|go\.work|Cargo\.toml|composer\.json|pubspec\.yaml|Package\.swift|Podfile|requirements\.txt|setup\.py|pyproject\.toml|Pipfile|Gemfile|node_modules|bower_components|__pycache__|__init__\.py|\.sln|Directory\.Build\.props)/?$`),
		// A framework, build tool or test runner, as the whole literal.
		regexp.MustCompile(`(?i)^(spring|spring-boot|django|flask|fastapi|laravel|symfony|nestjs|express|react|react-native|angular|vue|rails|maven|gradle|npm|yarn|pnpm|nuget|cargo|cocoapods|swiftpm|junit|testng|pytest|jest|vitest|mocha|xunit|nunit|phpunit|flutter|android|ios|kotlin|java|golang|python|typescript|javascript|csharp|php|swift|dart|ruby|rust)$`),
	}
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			for _, re := range ecosystem {
				if re.MatchString(v) {
					pos := fset.Position(lit.Pos())
					rel, _ := filepath.Rel(filepath.Dir(root), pos.Filename)
					t.Errorf("%s:%d: core names an ecosystem in %q; register it from the language pack or the extension's plug-in instead (ADR 0025)", filepath.ToSlash(rel), pos.Line, v)
					break
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
