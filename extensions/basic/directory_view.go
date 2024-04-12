package basic

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/util"
)

func directoryView(results *core.Results) *core.View {
	return util.GenericView(util.GetDistinctColumnsFrom(results.StatsByDirectory), results.StatsByDirectory)
}
