package declbased

import (
	"path/filepath"
	"strings"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
)

type componentLinker struct {
	Strategy string
	root     string
	// The languages' own readings of their imports, asked before an import
	// is read as a path. A func, because packs register theirs after this
	// linker is made.
	resolvers func() []component.ImportResolver
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
	for _, fr := range allFileResults {
		fileDirs[fr.Name] = fr.Directory
	}
	dirs := component.NewDirIndex(fileDirs)

	var resolvers []component.ImportResolver
	if c.resolvers != nil {
		resolvers = c.resolvers()
	}
	for _, r := range resolvers {
		r.Prepare(c.root, allFileResults)
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
				// The language's own reading first: what its manifests and
				// its rules say an import means. Only when none answers is it
				// worth guessing from the shape of the tree.
				if resolved, ok := resolveByLanguage(resolvers, snippet.File, fileDirs[snippet.File], snippet.Value, dirs); ok {
					snippet.Value = resolved
					continue
				}
				original := snippet.Value
				snippet.Value = resolveImport(fileDirs[snippet.File], snippet.Value, dirs, declaredComponents)

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
						if best := dirs.ShortestEndingIn(snippet.Value); best != "" {
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

// resolveByLanguage is the first registered resolver's answer, if any has
// one.
func resolveByLanguage(resolvers []component.ImportResolver, fromFile, fromDir, imp string, dirs component.Dirs) (string, bool) {
	for _, r := range resolvers {
		if resolved, ok := r.Resolve(fromFile, fromDir, imp, dirs); ok {
			return resolved, true
		}
	}
	return "", false
}

// resolveImport reads an import as a path, whatever language wrote it:
// relative to the importing file, as the tail of a directory, or dotted.
func resolveImport(importingFileDir, importValue string, dirs component.Dirs, declaredComponents map[string]bool) string {
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

	// 1. Relative paths: "./foo", "../bar"
	if strings.HasPrefix(importValue, ".") {
		resolved := filepath.Clean(filepath.Join(importingFileDir, importValue))
		resolved = strings.ReplaceAll(resolved, "\\", "/")

		// If the resolved directory itself exists, return it
		if dirs.Has(resolved) {
			return resolved
		}

		// If it's a file inside a known directory, resolved points to a file,
		// resolve to its containing directory
		parentDir := filepath.Dir(resolved)
		parentDir = strings.ReplaceAll(parentDir, "\\", "/")
		if dirs.Has(parentDir) {
			return parentDir
		}
		return resolved
	}

	// 2. A path naming a directory wherever the import root sits:
	// "components/foo" is the directory ending in it, or its parent when
	// the path names a file.
	if strings.Contains(importValue, "/") || strings.Contains(importValue, "\\") {
		cleaned := strings.ReplaceAll(importValue, "\\", "/")

		// Check if cleaned path itself is a suffix of any known directory
		bestMatch := dirs.ShortestEndingIn(cleaned)
		if bestMatch != "" {
			return bestMatch
		}

		// Check if the parent of cleaned path is a suffix of any known directory
		parent := filepath.Dir(cleaned)
		parent = strings.ReplaceAll(parent, "\\", "/")
		if parent != "." && parent != "/" {
			bestMatch = dirs.ShortestEndingIn(parent)
		}
		if bestMatch != "" {
			return bestMatch
		}
	}

	// 3. Slash-separated absolute imports: any import already written as a
	// path. Running one through the dotted branch below would split
	// `github.com` into two directories that never existed. The longest tail
	// of the path that names a directory is the component: longest first, so
	// a last segment that happens to match some unrelated folder cannot win
	// over the real match. A language whose imports carry more than a path
	// -- Go's module prefix -- resolves them itself and never reaches here,
	// because this guess also matches third-party packages.
	if strings.Contains(importValue, "/") && !strings.HasPrefix(importValue, ".") {
		cleaned := strings.Trim(strings.ReplaceAll(importValue, "\\", "/"), "/")
		segments := strings.Split(cleaned, "/")
		for start := 0; start < len(segments); start++ {
			if best := dirs.ShortestEndingIn(strings.Join(segments[start:], "/")); best != "" {
				return best
			}
		}
	}

	// 4. Dotted absolute imports: "my_package.sub_package"
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
		if best := dirs.ShortestEndingIn(slashed); best != "" {
			return best
		}

		parentDir := filepath.Dir(slashed)
		parentDir = strings.ReplaceAll(parentDir, "\\", "/")
		if best := dirs.ShortestEndingIn(parentDir); best != "" {
			return best
		}

		if dirs.Has(parentDir) {
			return parentDir
		}
		return slashed
	}

	return importValue
}
