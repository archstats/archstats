package declbased

import (
	"github.com/archstats/archstats/core"
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
