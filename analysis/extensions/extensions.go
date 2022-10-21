package extensions

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/RyanSusana/archstats/analysis/extensions/indentation"
	"github.com/RyanSusana/archstats/analysis/extensions/regex"
)

func BuiltInExtension(in string) (analysis.Extension, error) {
	switch in {
	case "indentation":
		return &indentation.Analyzer{}, nil
	default:
		return regex.BuiltInRegexExtension(in)
	}
}

func BuiltInExtensions() []string {
	return regex.AvailableExtensions()
}
