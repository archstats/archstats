package navigation

import (
	r "github.com/archstats/archstats/extensions/navigation/recogniser"
	"github.com/archstats/archstats/extensions/navigation/recogniser/dart"
	"github.com/archstats/archstats/extensions/navigation/recogniser/dotnet"
	"github.com/archstats/archstats/extensions/navigation/recogniser/ecmascript"
	gorec "github.com/archstats/archstats/extensions/navigation/recogniser/golang"
	"github.com/archstats/archstats/extensions/navigation/recogniser/jvm"
	phprec "github.com/archstats/archstats/extensions/navigation/recogniser/php"
	"github.com/archstats/archstats/extensions/navigation/recogniser/prisma"
	pyrec "github.com/archstats/archstats/extensions/navigation/recogniser/python"
	"github.com/archstats/archstats/extensions/navigation/recogniser/swift"
)

// plugins are every recogniser plug-in, in the order engine.Recognisers
// gives them.
func plugins() []r.Plugin {
	return []r.Plugin{jvm.Plugin(), dotnet.Plugin(), gorec.Plugin(), ecmascript.Plugin(), pyrec.Plugin(), phprec.Plugin(), swift.Plugin(), dart.Plugin(), prisma.Plugin()}
}
