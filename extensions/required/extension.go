package required

import (
	"github.com/RyanSusana/archstats/analysis"
)

func Extension() analysis.Extension {
	return &requiredExtensions{}
}

type requiredExtensions struct {
}

func (r *requiredExtensions) Init(settings analysis.Analyzer) error {
	settings.RegisterFileResultsEditor(&componentLinker{})
	return nil
}
