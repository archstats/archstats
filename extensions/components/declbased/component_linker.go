package declbased

import (
	"path/filepath"
	"strings"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
)

type componentLinker struct {
	Strategy string
	root     string
	aliases  *aliasMap
}

func (c *componentLinker) Init(settings core.Analyzer) error {
	return nil
}

func (c *componentLinker) interfaceAssertions() core.FileResultsEditor {
	return c
}

func (c *componentLinker) EditFileResults(allFileResults []*file.Results) {
	// Pre-map file name to its directory path
	fileDirs := make(map[string]string, len(allFileResults))
	names := make([]string, 0, len(allFileResults))
	for _, fr := range allFileResults {
		fileDirs[fr.Name] = fr.Directory
		names = append(names, fr.Name)
	}
	if c.aliases == nil {
		c.aliases = readAliasesFrom(c.root, names)
	}

	// 1. Handle directory-based component strategy
	if c.Strategy == "directory" {
		for _, fileResult := range allFileResults {
			for _, snippet := range fileResult.Snippets {
				snippet.Component = fileResult.Directory
			}
		}
	}

	allSnippets := lo.FlatMap(allFileResults, func(fileResult *file.Results, idx int) []*file.Snippet {
		return fileResult.Snippets
	})

	// 2. Handle declared and fallback strategies
	if c.Strategy == "declared" || c.Strategy == "fallback" {
		componentDeclarations := lo.Filter(allSnippets, func(snippet *file.Snippet, idx int) bool {
			return snippet.Type == file.ComponentDeclaration
		})
		snippetsByFile := lo.GroupBy(allSnippets, file.ByFile)
		componentDeclarationsByFile := lo.GroupBy(componentDeclarations, file.ByFile)

		// Files WITH explicit component declarations
		for fileName, componentDeclarationSnippets := range componentDeclarationsByFile {
			theComponent := componentDeclarationSnippets[0].Value
			snippets := snippetsByFile[fileName]
			for _, theSnippet := range snippets {
				theSnippet.Component = theComponent
			}
		}

		// Files WITHOUT explicit component declarations
		filesWithUnknownComponent := lo.Without(lo.Keys(snippetsByFile), lo.Keys(componentDeclarationsByFile)...)
		for _, fileName := range filesWithUnknownComponent {
			snippets := snippetsByFile[fileName]

			var componentName string
			if c.Strategy == "fallback" {
				componentName = fileDirs[fileName]
			} else {
				componentName = "Unknown"
			}

			for _, theSnippet := range snippets {
				theSnippet.Component = componentName
			}
		}
	}

	// 3. Resolve imports if strategy is "directory" or "fallback"
	if c.Strategy == "directory" || c.Strategy == "fallback" {
		declaredComponents := make(map[string]bool)
		// The components that will actually exist: the ones snippets have
		// been assigned to by the step above. A directory the walker saw is
		// not necessarily a component -- django-oscar's template trees hold
		// thousands of files and produce no snippets at all -- and the two
		// must agree, or a dynamic lookup resolves to a directory here and
		// is then reported as unresolvable by the view.
		actualComponents := make(map[string]bool)
		for _, snippet := range allSnippets {
			if snippet.Type == file.ComponentDeclaration {
				declaredComponents[snippet.Value] = true
			}
			if snippet.Component != "" {
				actualComponents[snippet.Component] = true
			}
		}

		for _, snippet := range allSnippets {
			// A type-only import resolves to a component exactly as an
			// ordinary one does; only what it counts for differs.
			if snippet.Type == file.ComponentImport || snippet.Type == file.ComponentImportTypeOnly || snippet.Type == file.ComponentImportDynamic {
				// The project's own vocabulary first. Only when nothing claims
				// the name is it worth guessing from the shape of the tree.
				if c.aliases != nil {
					if resolved := c.aliases.resolve(snippet.Value, fileDirs); resolved != "" {
						snippet.Value = resolved
						continue
					}
				}
				original := snippet.Value
				snippet.Value = resolveImport(fileDirs[snippet.File], snippet.Value, fileDirs, declaredComponents)

				// A dynamic lookup may name a thing rather than a path.
				// Django's `get_model("catalogue", "Product")` takes an app
				// label, and the app it means is a directory: django-oscar
				// keeps `catalogue` at `src/oscar/apps/catalogue`. A single
				// bare segment goes nowhere through the branches above, which
				// all want a dot or a slash, so 469 of Oscar's dynamic edges
				// -- 136 of them to `catalogue` alone -- resolved to nothing.
				//
				// Deliberately narrow: one segment, no dot, no slash, and
				// only for a dynamic lookup. An ordinary `import os` must
				// never be captured by a directory that happens to be called
				// os, and is not, because it never reaches here.
				if snippet.Type == file.ComponentImportDynamic {
					if !actualComponents[snippet.Value] && !strings.ContainsAny(snippet.Value, "./\\") {
						if best := shortestDirEndingIn(snippet.Value, fileDirs); best != "" {
							snippet.Value = best
						}
					}
					// Nothing in this codebase answers to it, so the resolver
					// has produced a guess rather than an answer -- and
					// `nowhere.at.all` comes back as `nowhere/at/all`, which
					// appears in no source file and cannot be searched for.
					// What the code wrote is the only honest thing to report.
					if !actualComponents[snippet.Value] {
						snippet.Value = original
					}
				}
			}
		}
	}
}

// shortestDirEndingIn finds the directory a path names, wherever the import
// root happens to sit in the tree. The shortest wins: between `src/acme/db`
// and `vendor/other/src/acme/db`, the first is the one the code means.
func shortestDirEndingIn(path string, fileDirs map[string]string) string {
	if path == "" || path == "." || path == "/" {
		return ""
	}
	best := ""
	for _, dir := range fileDirs {
		if dir == path || strings.HasSuffix(dir, "/"+path) {
			if best == "" || len(dir) < len(best) {
				best = dir
			}
		}
	}
	return best
}

func resolveImport(importingFileDir, importValue string, fileDirs map[string]string, declaredComponents map[string]bool) string {
	// 0. If it matches a declared component, keep it as is
	if declaredComponents[importValue] {
		return importValue
	}
	if strings.Contains(importValue, ".") && !strings.HasPrefix(importValue, ".") {
		parts := strings.Split(importValue, ".")
		for i := len(parts) - 1; i > 0; i-- {
			prefix := strings.Join(parts[:i], ".")
			if declaredComponents[prefix] {
				return prefix
			}
		}
	}

	// 1. Python-style dot-relative imports (e.g. ".db", "..utils.crypto")
	if strings.HasPrefix(importValue, ".") && !strings.Contains(importValue, "/") && !strings.Contains(importValue, "\\") {
		// Count leading dots
		dotsCount := 0
		for dotsCount < len(importValue) && importValue[dotsCount] == '.' {
			dotsCount++
		}

		// The rest of the import value after leading dots
		rest := importValue[dotsCount:]
		restSlashed := strings.ReplaceAll(rest, ".", "/")

		// Calculate base directory by going up dotsCount - 1 levels
		baseDir := importingFileDir
		for i := 0; i < dotsCount-1; i++ {
			baseDir = filepath.Dir(baseDir)
		}

		resolved := filepath.Join(baseDir, restSlashed)
		resolved = strings.ReplaceAll(resolved, "\\", "/")

		// If the resolved directory itself exists in fileDirs, return it
		for _, dir := range fileDirs {
			if resolved == dir {
				return resolved
			}
		}

		// If it's a file inside a known directory, resolve to its containing directory
		parentDir := filepath.Dir(resolved)
		parentDir = strings.ReplaceAll(parentDir, "\\", "/")
		for _, dir := range fileDirs {
			if parentDir == dir {
				return parentDir
			}
		}
		return resolved
	}

	// 2. Standard relative paths (JS/TS, e.g. "./foo", "../bar")
	if strings.HasPrefix(importValue, ".") {
		resolved := filepath.Clean(filepath.Join(importingFileDir, importValue))
		resolved = strings.ReplaceAll(resolved, "\\", "/")

		// If the resolved directory itself exists in fileDirs, return it
		for _, dir := range fileDirs {
			if resolved == dir {
				return resolved
			}
		}

		// If it's a file inside a known directory, resolved points to a file,
		// resolve to its containing directory
		parentDir := filepath.Dir(resolved)
		parentDir = strings.ReplaceAll(parentDir, "\\", "/")
		for _, dir := range fileDirs {
			if parentDir == dir {
				return parentDir
			}
		}
		return resolved
	}

	// 3. JS/TS Alias and Path mappings (e.g. "@/components/foo", "~/utils/bar", "@components/baz", "components/foo")
	if strings.HasPrefix(importValue, "@") || strings.HasPrefix(importValue, "~") || strings.Contains(importValue, "/") || strings.Contains(importValue, "\\") {
		cleaned := importValue
		cleaned = strings.ReplaceAll(cleaned, "\\", "/")
		if strings.HasPrefix(cleaned, "@/") || strings.HasPrefix(cleaned, "~/") {
			cleaned = cleaned[2:]
		} else if strings.HasPrefix(cleaned, "@") || strings.HasPrefix(cleaned, "~") {
			cleaned = cleaned[1:]
		}

		// Check if cleaned path itself is a suffix of any known directory
		var bestMatch string
		for _, dir := range fileDirs {
			if dir == cleaned || strings.HasSuffix(dir, "/"+cleaned) {
				if bestMatch == "" || len(dir) < len(bestMatch) {
					bestMatch = dir
				}
			}
		}
		if bestMatch != "" {
			return bestMatch
		}

		// Check if the parent of cleaned path is a suffix of any known directory
		parent := filepath.Dir(cleaned)
		parent = strings.ReplaceAll(parent, "\\", "/")
		if parent != "." && parent != "/" {
			for _, dir := range fileDirs {
				if dir == parent || strings.HasSuffix(dir, "/"+parent) {
					if bestMatch == "" || len(dir) < len(bestMatch) {
						bestMatch = dir
					}
				}
			}
		}
		if bestMatch != "" {
			return bestMatch
		}
	}

	// 3b. Slash-separated absolute imports: Go, and any import already
	// written as a path. The module prefix is not part of the tree --
	// `github.com/acme/thing/core/file` lives at `core/file` -- and running
	// it through the dotted branch below would split `github.com` into two
	// directories that never existed, which is why Go resolved to nothing at
	// all. The longest tail of the path that names a directory is the
	// component: longest first, so a module whose last segment happens to
	// match some unrelated folder cannot win over the real match.
	if strings.Contains(importValue, "/") && !strings.HasPrefix(importValue, ".") {
		cleaned := strings.Trim(strings.ReplaceAll(importValue, "\\", "/"), "/")
		segments := strings.Split(cleaned, "/")
		for start := 0; start < len(segments); start++ {
			if best := shortestDirEndingIn(strings.Join(segments[start:], "/"), fileDirs); best != "" {
				return best
			}
		}
	}

	// 4. Python/Java dot-separated absolute imports (e.g. "my_package.sub_package")
	if strings.Contains(importValue, ".") && !strings.HasPrefix(importValue, ".") {
		slashed := strings.ReplaceAll(importValue, ".", "/")
		slashed = strings.ReplaceAll(slashed, "\\", "/")

		// An absolute import names the package from the root of the import
		// path, which is almost never the root of the repository: `src/`
		// layouts, `app/`, a monorepo package folder. Matched by equality
		// alone, `acme.billing` failed to find `src/acme/billing` and every
		// absolute intra-package import in a modern Python project was lost.
		// The shortest match wins, so a vendored copy deeper in the tree
		// cannot outrank the real one.
		if best := shortestDirEndingIn(slashed, fileDirs); best != "" {
			return best
		}

		parentDir := filepath.Dir(slashed)
		parentDir = strings.ReplaceAll(parentDir, "\\", "/")
		if best := shortestDirEndingIn(parentDir, fileDirs); best != "" {
			return best
		}

		for _, dir := range fileDirs {
			if parentDir == dir {
				return parentDir
			}
		}
		return slashed
	}

	return importValue
}
