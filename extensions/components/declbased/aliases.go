package declbased

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// What a project says its own import names mean.
//
// An enterprise JavaScript or TypeScript codebase does not import by relative
// path. It imports `@acme/shared` and `~/components/Chat`, and what those
// mean is written down: in the `name` of each workspace's package.json, and
// in the `paths` of its tsconfig. Guessing at them by matching directory
// suffixes happens to work on a single-package repo and fails on every
// monorepo — on a four-package fixture, three of five real edges were
// dropped in silence, and LibreChat's client writes 2,050 of its imports in
// the one style that has to be read rather than guessed.
//
// Read once per run, before any import is resolved.

type aliasEntry struct {
	// The import prefix this matches: "@acme/shared" exactly, or "@acme/ui/"
	// for everything beneath it.
	prefix string
	exact  bool
	// Where it points. More than one, because a package may be imported by
	// its root while its code lives in `src`, and only the tree can say.
	dirs []string
}

type aliasMap struct {
	entries []aliasEntry
}

// skipDir keeps the walk to a project's own source.
var skipDir = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true,
	"out": true, "target": true, "vendor": true, ".next": true, ".nuxt": true,
	"coverage": true, "__pycache__": true, ".venv": true, "venv": true,
}

func readAliases(root string) *aliasMap {
	m := &aliasMap{}
	if root == "" {
		return m
	}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch d.Name() {
		case "package.json":
			m.readPackage(root, p)
		case "tsconfig.json", "jsconfig.json":
			m.readTSConfig(root, p)
		}
		return nil
	})
	// Longest prefix first, so "@acme/ui/" is tried before "@acme/".
	sort.SliceStable(m.entries, func(i, j int) bool {
		return len(m.entries[i].prefix) > len(m.entries[j].prefix)
	})
	return m
}

type packageManifest struct {
	Name   string `json:"name"`
	Main   string `json:"main"`
	Module string `json:"module"`
	Source string `json:"source"`
	Types  string `json:"types"`
}

// A workspace package is imported by the name it gives itself.
func (m *aliasMap) readPackage(root, manifestPath string) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return
	}
	var pkg packageManifest
	if json.Unmarshal(raw, &pkg) != nil || pkg.Name == "" {
		return
	}
	pkgDir := relDir(root, filepath.Dir(manifestPath))

	// Where its code actually sits. An entry point of "src/index.ts" says the
	// source is a directory down, which is where a subpath import lands too.
	dirs := []string{}
	for _, entry := range []string{pkg.Source, pkg.Module, pkg.Main, pkg.Types} {
		if entry == "" {
			continue
		}
		if d := joinRel(pkgDir, path.Dir(entry)); d != "" && d != pkgDir {
			dirs = appendUnique(dirs, d)
		}
	}
	dirs = appendUnique(dirs, joinRel(pkgDir, "src"))
	dirs = appendUnique(dirs, pkgDir)

	m.entries = append(m.entries,
		aliasEntry{prefix: pkg.Name, exact: true, dirs: dirs},
		aliasEntry{prefix: pkg.Name + "/", dirs: dirs},
	)
}

type tsConfig struct {
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

// tsconfig allows comments and trailing commas, which encoding/json does not.
var (
	blockComments = regexp.MustCompile(`(?s)/\*.*?\*/`)
	lineComments  = regexp.MustCompile(`(?m)^\s*//.*$`)
	trailingComma = regexp.MustCompile(`,(\s*[}\]])`)
)

func (m *aliasMap) readTSConfig(root, configPath string) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	cleaned := trailingComma.ReplaceAllString(
		lineComments.ReplaceAllString(blockComments.ReplaceAllString(string(raw), ""), ""), "$1")

	var cfg tsConfig
	if json.Unmarshal([]byte(cleaned), &cfg) != nil || len(cfg.CompilerOptions.Paths) == 0 {
		return
	}

	configDir := relDir(root, filepath.Dir(configPath))
	base := joinRel(configDir, cfg.CompilerOptions.BaseURL)

	for pattern, targets := range cfg.CompilerOptions.Paths {
		// "*" alone maps everything and would swallow every third-party
		// import; it is a resolution fallback, not a name for anything here.
		if pattern == "*" {
			continue
		}
		var dirs []string
		for _, target := range targets {
			target = strings.TrimSuffix(target, "*")
			dirs = appendUnique(dirs, joinRel(base, path.Dir(path.Join(target, "x"))))
		}
		if strings.HasSuffix(pattern, "*") {
			m.entries = append(m.entries, aliasEntry{prefix: strings.TrimSuffix(pattern, "*"), dirs: dirs})
		} else {
			m.entries = append(m.entries, aliasEntry{prefix: pattern, exact: true, dirs: dirs})
		}
	}
}

// resolve turns an import written in the project's own vocabulary into the
// directory it names, or "" when no alias claims it — which is the common and
// correct answer for a third-party package.
func (m *aliasMap) resolve(importValue string, fileDirs map[string]string) string {
	for _, entry := range m.entries {
		var rest string
		if entry.exact {
			if importValue != entry.prefix {
				continue
			}
		} else {
			if !strings.HasPrefix(importValue, entry.prefix) {
				continue
			}
			rest = strings.TrimPrefix(importValue, entry.prefix)
		}
		for _, dir := range entry.dirs {
			candidate := joinRel(dir, rest)
			if isKnownDir(candidate, fileDirs) {
				return candidate
			}
			// The import named a file; its component is the directory holding it.
			if parent := path.Dir(candidate); isKnownDir(parent, fileDirs) {
				return parent
			}
		}
	}
	return ""
}

func isKnownDir(candidate string, fileDirs map[string]string) bool {
	if candidate == "" || candidate == "." {
		return false
	}
	for _, dir := range fileDirs {
		if dir == candidate {
			return true
		}
	}
	return false
}

func relDir(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return ""
	}
	return rel
}

func joinRel(base, rest string) string {
	rest = strings.TrimPrefix(filepath.ToSlash(rest), "./")
	if rest == "." {
		rest = ""
	}
	joined := path.Join(base, rest)
	if joined == "." {
		return ""
	}
	return joined
}

func appendUnique(list []string, value string) []string {
	if value == "" {
		return list
	}
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
