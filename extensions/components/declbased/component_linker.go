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
		for _, snippet := range allSnippets {
			if snippet.Type == file.ComponentDeclaration {
				declaredComponents[snippet.Value] = true
			}
		}

		for _, snippet := range allSnippets {
			if snippet.Type == file.ComponentImport {
				snippet.Value = resolveImport(fileDirs[snippet.File], snippet.Value, fileDirs, declaredComponents)
			}
		}
	}
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

	// 4. Python/Java dot-separated absolute imports (e.g. "my_package.sub_package")
	if strings.Contains(importValue, ".") && !strings.HasPrefix(importValue, ".") {
		slashed := strings.ReplaceAll(importValue, ".", "/")
		slashed = strings.ReplaceAll(slashed, "\\", "/")

		// If the slashed directory itself exists in fileDirs, return it
		for _, dir := range fileDirs {
			if slashed == dir {
				return slashed
			}
		}

		// If it's a file inside a known directory, resolve to its containing directory
		parentDir := filepath.Dir(slashed)
		parentDir = strings.ReplaceAll(parentDir, "\\", "/")
		for _, dir := range fileDirs {
			if parentDir == dir {
				return parentDir
			}
		}
		return slashed
	}

	return importValue
}
