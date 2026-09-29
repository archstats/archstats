package common

import (
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/extensions/basic"
	"github.com/archstats/archstats/extensions/codesmells"
	"github.com/archstats/archstats/extensions/components"
	"github.com/archstats/archstats/extensions/components/cycles"
	"github.com/archstats/archstats/extensions/components/declbased"
	"github.com/archstats/archstats/extensions/deployables"
	"github.com/archstats/archstats/extensions/git"
	"github.com/archstats/archstats/extensions/indentations"
	"github.com/archstats/archstats/extensions/lines"
	"github.com/archstats/archstats/extensions/matrix"
	"github.com/archstats/archstats/extensions/mobile"
	"github.com/archstats/archstats/extensions/regex"
	"github.com/archstats/archstats/extensions/rules"
	"github.com/archstats/archstats/extensions/treesitter/csharp"
	"github.com/archstats/archstats/extensions/treesitter/dart"
	"github.com/archstats/archstats/extensions/treesitter/golang"
	"github.com/archstats/archstats/extensions/treesitter/java"
	"github.com/archstats/archstats/extensions/treesitter/javascript"
	"github.com/archstats/archstats/extensions/treesitter/kotlin"
	"github.com/archstats/archstats/extensions/treesitter/objc"
	"github.com/archstats/archstats/extensions/treesitter/php"
	"github.com/archstats/archstats/extensions/treesitter/python"
	"github.com/archstats/archstats/extensions/treesitter/swift"
	"github.com/archstats/archstats/extensions/treesitter/typescript"
	"github.com/gobwas/glob"
)

func Optional() []*config.CLIConfiguredExtension {
	gitExt := git.CLIExtension()
	gitExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		// Not ctx.HasPath(".git"): that sees only a root that is itself a
		// repository, so a workspace holding several checkouts got no git
		// data at all, however many repositories were inside it.
		return git.HasRepos(ctx.RootDir)
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
		// A single-file component's script is read by this pack too, and a
		// Vue or Svelte app written in plain JavaScript may have no .ts file.
		return ctx.HasFileExtension(".ts") || ctx.HasFileExtension(".tsx") || ctx.HasFileExtension(".mts") || ctx.HasFileExtension(".cts") ||
			ctx.HasFileExtension(".vue") || ctx.HasFileExtension(".svelte")
	}

	goExt := config.CreateEmptyCLIExtension("go", &golang.Extension{})
	goExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".go")
	}

	pythonExt := config.CreateEmptyCLIExtension("python", &python.Extension{})
	pythonExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".py")
	}

	phpExt := config.CreateEmptyCLIExtension("php", &php.Extension{})
	phpExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".php")
	}

	swiftExt := config.CreateEmptyCLIExtension("swift", &swift.Extension{})
	swiftExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".swift")
	}

	// Headers alone are C as often as Objective-C; an implementation file
	// says which.
	objcExt := config.CreateEmptyCLIExtension("objc", &objc.Extension{})
	objcExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".m") || ctx.HasFileExtension(".mm")
	}

	dartExt := config.CreateEmptyCLIExtension("dart", &dart.Extension{})
	dartExt.DiscoveryTrigger = func(ctx *config.DiscoveryContext) bool {
		return ctx.HasFileExtension(".dart")
	}

	extensions := []*config.CLIConfiguredExtension{
		gitExt,
		javaExt,
		csharpExt,
		kotlinExt,
		javascriptExt,
		typescriptExt,
		pythonExt,
		goExt,
		phpExt,
		swiftExt,
		objcExt,
		dartExt,
		config.CreateEmptyCLIExtension("cycles", cycles.Extension()),
	}
	for name, extension := range regex.GetLanguageExtensions() {
		// Skip the languages the tree-sitter extensions above supersede.
		// Their regex definitions stay in the yaml as a record of what the
		// pack has to match, and for anybody running the regex extension by
		// name.
		if name == "kotlin" || name == "javascript" || name == "typescript" || name == "python" || name == "go" || name == "php" {
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
		matrix.CLIExtension(),
		config.CreateEmptyCLIExtension("rules", rules.Extension()),
		config.CreateEmptyCLIExtension("deployables", deployables.Extension()),
		config.CreateEmptyCLIExtension("mobile", mobile.Extension()),
	}
}
