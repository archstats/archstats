package common

import (
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/extensions/basic"
	"github.com/archstats/archstats/extensions/codesmells"
	"github.com/archstats/archstats/extensions/components"
	"github.com/archstats/archstats/extensions/components/cycles"
	"github.com/archstats/archstats/extensions/components/declbased"
	"github.com/archstats/archstats/extensions/git"
	"github.com/archstats/archstats/extensions/indentations"
	"github.com/archstats/archstats/extensions/lines"
	"github.com/archstats/archstats/extensions/matrix"
	"github.com/archstats/archstats/extensions/regex"
	"github.com/archstats/archstats/extensions/treesitter/csharp"
	"github.com/archstats/archstats/extensions/treesitter/java"
	"github.com/archstats/archstats/extensions/treesitter/javascript"
	"github.com/archstats/archstats/extensions/treesitter/kotlin"
	"github.com/archstats/archstats/extensions/treesitter/python"
	"github.com/archstats/archstats/extensions/treesitter/typescript"
	"github.com/gobwas/glob"
)

func Optional() []*config.CLIConfiguredExtension {
	gitExt := git.CLIExtension()
	gitExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasPath(".git")
	}

	javaExt := java.CLIExtension()
	javaExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".java")
	}

	csharpExt := config.CreateEmptyCLIExtension("csharp", &csharp.Extension{})
	csharpExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".cs")
	}

	kotlinExt := config.CreateEmptyCLIExtension("kotlin", &kotlin.Extension{})
	kotlinExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".kt")
	}

	javascriptExt := config.CreateEmptyCLIExtension("javascript", &javascript.Extension{})
	javascriptExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".js") || ctx.HasFileExtension(".jsx") || ctx.HasFileExtension(".mjs") || ctx.HasFileExtension(".cjs")
	}

	typescriptExt := config.CreateEmptyCLIExtension("typescript", &typescript.Extension{})
	typescriptExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".ts") || ctx.HasFileExtension(".tsx") || ctx.HasFileExtension(".mts") || ctx.HasFileExtension(".cts")
	}

	pythonExt := config.CreateEmptyCLIExtension("python", &python.Extension{})
	pythonExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".py")
	}

	extensions := []*config.CLIConfiguredExtension{
		gitExt,
		javaExt,
		csharpExt,
		kotlinExt,
		javascriptExt,
		typescriptExt,
		pythonExt,
		config.CreateEmptyCLIExtension("cycles", cycles.Extension()),
	}
	for name, extension := range regex.GetLanguageExtensions() {
		// Skip kotlin, javascript, typescript, python — the tree-sitter extensions above supersede them
		if name == "kotlin" || name == "javascript" || name == "typescript" || name == "python" {
			continue
		}
		cliExt := config.CreateEmptyCLIExtension(name, extension)

		if regexExt, ok := extension.(*regex.Extension); ok && regexExt.GlobString != "" {
			g, err := glob.Compile(regexExt.GlobString)
			if err == nil {
				cliExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
					for _, file := range ctx.Files {
						if g.Match(file) {
							return true
						}
					}
					return false
				}
			}
		}

		extensions = append(extensions, cliExt)
	}
	return extensions
}

func AlwaysEnabled() []*config.CLIConfiguredExtension {
	return []*config.CLIConfiguredExtension{
		indentations.CLIExtension(),
		config.CreateEmptyCLIExtension("basic", basic.Extension()),
		config.CreateEmptyCLIExtension("components", components.Extension()),
		config.CreateEmptyCLIExtension("lines", lines.Extension()),
		declbased.CLIExtension(),
		config.CreateEmptyCLIExtension("codesmells", codesmells.Extension()),
		config.CreateEmptyCLIExtension("matrix", matrix.Extension()),
	}
}

