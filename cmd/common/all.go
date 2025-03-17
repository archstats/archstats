package common

import (
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/extensions/basic"
	"github.com/archstats/archstats/extensions/components"
	"github.com/archstats/archstats/extensions/components/cycles"
	"github.com/archstats/archstats/extensions/components/declbased"
	"github.com/archstats/archstats/extensions/git"
	"github.com/archstats/archstats/extensions/indentations"
	"github.com/archstats/archstats/extensions/lines"
	"github.com/archstats/archstats/extensions/regex"
	"github.com/archstats/archstats/extensions/treesitter/csharp"
	"github.com/archstats/archstats/extensions/treesitter/java"
)

func Optional() []*config.CLIConfiguredExtension {
	extensions := []*config.CLIConfiguredExtension{
		java.Ext(),
		config.CreateEmptyCLIExtension("csharp", &csharp.Extension{}),
		config.CreateEmptyCLIExtension("git", git.Extension()),
		config.CreateEmptyCLIExtension("cycles", cycles.Extension()),
	}
	for name, extension := range regex.GetLanguageExtensions() {
		extensions = append(extensions, config.CreateEmptyCLIExtension(name, extension))
	}
	return extensions
}

func AlwaysEnabled() []*config.CLIConfiguredExtension {
	return []*config.CLIConfiguredExtension{
		config.CreateEmptyCLIExtension("indentations", indentations.FourTabs()),
		config.CreateEmptyCLIExtension("basic", basic.Extension()),
		config.CreateEmptyCLIExtension("components", components.Extension()),
		config.CreateEmptyCLIExtension("lines", lines.Extension()),
		config.CreateEmptyCLIExtension("declbased", declbased.Extension()),
	}
}
