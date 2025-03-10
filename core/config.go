package core

import (
	"github.com/archstats/archstats/core/component"
	"github.com/archstats/archstats/core/file"
)

// Config represents the settings for an analysis.
type Config struct {
	// RootPath is the path to the root directory of the codebase to analyze. If not specified, the current working directory is used.
	RootPath string
	// Extensions are the extensions to use for the analysis.
	Extensions []Extension
}

// Extension represents an extension to the analysis. All Archstats extensions must implement this interface and live outside the core package
type Extension interface {
	Init(settings Analyzer) error
}
type FileAnalyzer interface {
	AnalyzeFile(file.File) *file.Results
}
type ComponentsAnalyzer interface {
	AnalyzeComponents(allResults []*file.Results) *component.Results
}
type FileResultsEditor interface {
	EditFileResults(all []*file.Results)
}
type ResultsEditor interface {
	EditResults(results *Results)
}
