package engine

import (
	"github.com/archstats/archstats/core/plugin"
	"github.com/archstats/archstats/extensions/basic"
	"github.com/archstats/archstats/extensions/cicd"
	"github.com/archstats/archstats/extensions/cicd/buildtool"
	"github.com/archstats/archstats/extensions/cicd/buildtool/dotnet"
	"github.com/archstats/archstats/extensions/cicd/buildtool/gomod"
	"github.com/archstats/archstats/extensions/cicd/buildtool/gradle"
	"github.com/archstats/archstats/extensions/cicd/buildtool/maven"
	"github.com/archstats/archstats/extensions/cicd/buildtool/node"
	"github.com/archstats/archstats/extensions/cicd/buildtool/pub"
	"github.com/archstats/archstats/extensions/cicd/buildtool/xcode"
	"github.com/archstats/archstats/extensions/codesmells"
	"github.com/archstats/archstats/extensions/components"
	"github.com/archstats/archstats/extensions/components/cycles"
	"github.com/archstats/archstats/extensions/components/declbased"
	"github.com/archstats/archstats/extensions/git"
	"github.com/archstats/archstats/extensions/indentations"
	"github.com/archstats/archstats/extensions/lines"
	"github.com/archstats/archstats/extensions/matrix"
	"github.com/archstats/archstats/extensions/mobile"
	"github.com/archstats/archstats/extensions/navigation"
	"github.com/archstats/archstats/extensions/navigation/recogniser"
	navdart "github.com/archstats/archstats/extensions/navigation/recogniser/dart"
	navdotnet "github.com/archstats/archstats/extensions/navigation/recogniser/dotnet"
	"github.com/archstats/archstats/extensions/navigation/recogniser/ecmascript"
	navgo "github.com/archstats/archstats/extensions/navigation/recogniser/golang"
	"github.com/archstats/archstats/extensions/navigation/recogniser/jvm"
	navphp "github.com/archstats/archstats/extensions/navigation/recogniser/php"
	"github.com/archstats/archstats/extensions/navigation/recogniser/prisma"
	navpython "github.com/archstats/archstats/extensions/navigation/recogniser/python"
	navswift "github.com/archstats/archstats/extensions/navigation/recogniser/swift"
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

// Optional is every extension a workspace switches on by having what it
// reads, or that is asked for by name, in the order they run.
func Optional() []*plugin.Descriptor {
	gitExt := git.CLIExtension()
	gitExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		// Not ctx.HasPath(".git"): that sees only a root that is itself a
		// repository, so a workspace holding several checkouts got no git
		// data at all, however many repositories were inside it.
		return git.HasRepos(ctx.RootDir)
	}

	javaExt := java.CLIExtension()
	javaExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".java")
	}

	csharpExt := plugin.Fixed("csharp", &csharp.Extension{})
	csharpExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".cs")
	}

	kotlinExt := plugin.Fixed("kotlin", &kotlin.Extension{})
	kotlinExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".kt")
	}

	javascriptExt := plugin.Fixed("javascript", &javascript.Extension{})
	javascriptExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".js") || ctx.HasFileExtension(".jsx") || ctx.HasFileExtension(".mjs") || ctx.HasFileExtension(".cjs")
	}

	typescriptExt := plugin.Fixed("typescript", &typescript.Extension{})
	typescriptExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		// A single-file component's script is read by this pack too, and a
		// Vue or Svelte app written in plain JavaScript may have no .ts file.
		return ctx.HasFileExtension(".ts") || ctx.HasFileExtension(".tsx") || ctx.HasFileExtension(".mts") || ctx.HasFileExtension(".cts") ||
			ctx.HasFileExtension(".vue") || ctx.HasFileExtension(".svelte")
	}

	goExt := plugin.Fixed("go", &golang.Extension{})
	goExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".go")
	}

	pythonExt := plugin.Fixed("python", &python.Extension{})
	pythonExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".py")
	}

	phpExt := plugin.Fixed("php", &php.Extension{})
	phpExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".php")
	}

	swiftExt := plugin.Fixed("swift", &swift.Extension{})
	swiftExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".swift")
	}

	// Headers alone are C as often as Objective-C; an implementation file
	// says which.
	objcExt := plugin.Fixed("objc", &objc.Extension{})
	objcExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".m") || ctx.HasFileExtension(".mm")
	}

	dartExt := plugin.Fixed("dart", &dart.Extension{})
	dartExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
		return ctx.HasFileExtension(".dart")
	}

	extensions := []*plugin.Descriptor{
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
		plugin.Fixed("cycles", cycles.Extension()),
	}
	for name, extension := range regex.GetLanguageExtensions() {
		// Skip the languages the tree-sitter extensions above supersede.
		// Their regex definitions stay in the yaml as a record of what the
		// pack has to match, and for anybody running the regex extension by
		// name.
		if name == "kotlin" || name == "javascript" || name == "typescript" || name == "python" || name == "go" || name == "php" {
			continue
		}
		cliExt := plugin.Fixed(name, extension)

		if regexExt, ok := extension.(*regex.Extension); ok && regexExt.GlobString != "" {
			g, err := glob.Compile(regexExt.GlobString)
			if err == nil {
				cliExt.DiscoveryTrigger = func(ctx *plugin.DiscoveryContext) bool {
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

// AlwaysEnabled is every extension that runs on every scan, in the order
// they run. Each call builds fresh instances.
func AlwaysEnabled() []*plugin.Descriptor {
	return []*plugin.Descriptor{
		indentations.CLIExtension(),
		plugin.Fixed("basic", basic.Extension()),
		plugin.Fixed("components", components.Extension()),
		plugin.Fixed("lines", lines.Extension()),
		declbased.CLIExtension(),
		plugin.Fixed("codesmells", codesmells.Extension()),
		matrix.CLIExtension(),
		plugin.Fixed("rules", rules.Extension()),
		plugin.Fixed("cicd", cicd.Extension(BuildTools()...)),
		plugin.Fixed("mobile", mobile.Extension()),
		plugin.Fixed("navigation", navigation.Extension(Recognisers()...)),
	}
}

// Recognisers are the language families navigation reads, one plug-in per
// family. JVM comes before PHP: both read XML, Hibernate and Spring before
// Doctrine and Symfony.
func Recognisers() []recogniser.Plugin {
	return []recogniser.Plugin{jvm.Plugin(), navdotnet.Plugin(), navgo.Plugin(), ecmascript.Plugin(), navpython.Plugin(), navphp.Plugin(), navswift.Plugin(), navdart.Plugin(), prisma.Plugin()}
}

// BuildTools are the build tools whose manifests CI/CD reads, one per
// ecosystem, in the order their applications are listed.
func BuildTools() []buildtool.Tool {
	return []buildtool.Tool{maven.Tool(), gradle.Tool(), dotnet.Tool(), node.Tool(), xcode.Tool(), pub.Tool(), gomod.Tool()}
}
