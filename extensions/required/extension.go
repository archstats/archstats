package required

import (
	"github.com/RyanSusana/archstats/core"
)

func Extension() core.Extension {
	return &requiredExtensions{}
}

type requiredExtensions struct {
}

func (r *requiredExtensions) Init(settings core.Analyzer) error {
	settings.RegisterFileResultsEditor(&componentLinker{})
	return nil
}
