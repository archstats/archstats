# 0025. Core Knows No Ecosystem; Extensions Take Ecosystems as Plug-ins

- **Status**: Accepted
- **Date**: 2026-10-29

## Context and Problem Statement

ADR 0001 drew the microkernel line: core walks, calls extensions and aggregates, and every language and ecosystem lives in `extensions/`. ADR 0023 then accepted that core had grown past that line. It held ten manifest readers in `core/module` (about 1,400 lines covering Maven, Gradle, Android plugin typing, csproj, Composer, npm, go.mod, Django, SwiftPM, Xcode with a property-list parser, and pubspec). `core/file` held every ecosystem's test, generated-code and vendored-code conventions and the CI/CD file names. `core/unit` stripped JavaScript extensions, knew TypeScript's `~/` and `@/` aliases, and split PHP's backslash. ADR 0023 turned down moving them because of two worries: nothing declared the order extensions would have to run in, and a large move would collide with work in flight.

Extensions had the same problem one level up. The component linker read go.mod, package.json and tsconfig and Python's leading dots itself. CI/CD parsed pom inheritance, build.gradle, .csproj and Aspire's Program.cs, and switched on module kinds to find mobile, desktop and runtime facts. Navigation reads routes, ORMs, DI wiring and HTTP clients for about ten languages in one package. "A new language is an extension" was true for parsing and false for everything built on top of it.

## Decision Drivers

- **A solid core**: core holds the model, the questions and the precedence between answers. Every answer that depends on a language, a framework or a build tool lives with that ecosystem.
- **One place per ecosystem**: adding or fixing Gradle, Django or Aspire touches that ecosystem's package and nothing shared.
- **No hidden ordering**: registration happens in `Init`, and core applies what was registered at the same lifecycle point it always did. No extension reads another's output.
- **Same snapshot**: the move must not change scan output.

## Considered Options

- **Keep ADR 0023's boundary** and document the leaks.
- **Registries in core, filled in `Init`.** Language packs register what they know about their own ecosystem. Domain extensions take their ecosystem-specific parts as plug-ins that the engine wires in.
- **One "ecosystems" extension** holding every manifest reader and convention. This just moves the multi-language package out of core.

## Decision Outcome

Chosen option: **registries in core, filled in `Init`**. Ordering is not a problem, because core still runs the readers and classifiers itself: it reads modules right after the walk and classifies each file during it. Only the code that answers comes from extensions.

1. **Core holds questions, not answers.**
   - `core/module`: the `Module` model, `Map`, the `Reader` and `Finisher` interfaces, and `Analyzer.RegisterModuleReader`. The map is read once after the walk, and `Analyzer.Modules()` serves it to editors.
   - `core/file`: roles, the system-kind vocabulary and a `Classifier` over registered `Conventions` (`Analyzer.RegisterConventions`). Core keeps only what holds in every language: `tests/`, `spec/`, `e2e/`, `.feature`, `vendor/` and `third_party/`, the cross-tool "Code generated … DO NOT EDIT." marker, and data formats. A lockfile counts as non-code by its system kind.
   - `core/unit`: resolves a `Ref`'s `Path` and `Unaliased`, which a pack fills in. A qualified ID splits before the unit's name, whatever separator the language uses.
   - `core/component`: `ImportResolver`, `Dirs` and `DirIndex`, registered with `Analyzer.RegisterImportResolver` and asked by the component linker before it reads an import as a path.
2. **A language pack owns its ecosystem.** It registers its manifest readers, its file conventions and its import resolver in `Init`, next to its parser. Code shared by a family lives in a family package that both packs register:
   - `treesitter/jvm` for Java and Kotlin;
   - `treesitter/ecmascript` for JavaScript and TypeScript;
   - `treesitter/apple` for Swift and Objective-C.

   `treesitter/common` holds only what works the same in every language.
3. **A domain extension stays neutral and takes ecosystems as plug-ins.**
   - CI/CD knows pipelines, containers, deployments and the file names of build tools. The manifests of Maven, Gradle, .NET, npm, Xcode, pub and Go are read by `extensions/cicd/buildtool/<tool>`, against the contract in `extensions/cicd/buildtool`.
   - `engine.BuildTools` lists the tools, and the catalogue passes them in.
   - A plug-in imports the contract, never the extension, so the extension's tests can import the plug-ins.
4. **The composition root wires it.** `engine` is the only package that names every language, tool and extension. `engine.Classifier()` gives tools outside a scan every convention in the catalogue.
5. **Where the line falls.** A domain extension may know its own domain's vocabulary: CI systems, Jenkins steps, container base images, and which file names belong to a build tool. Two things belong to a plug-in or a pack instead:
   - parsing a language's source or an ecosystem's manifest;
   - branching on a language or a module kind.

### Consequences

- **Good**: adding an ecosystem is a pack, or a plug-in plus one line in `engine`. Core doesn't change. ADR 0001's promise holds for manifests, conventions and imports, not just parsing.
- **Good**: a scan reads only the conventions and manifests of the packs it runs, and each import resolver answers only for its own language's files. A package.json name can no longer claim a Python import.
- **Good**: fifteen checkouts in `e2eTest/temp_testdata` (C#, Java, Kotlin, JS, TS, Python and Go, and the polyglot OTel demo, eShop and petclinic) scan to the same snapshot as before. The only exceptions are short-cycle columns and PageRank, which already differed between two runs of the same binary.
- **Bad**: an ecosystem without a pack (Rust, Ruby) only gets what core and the domain extensions know. Its build files are still named by CI/CD, but nothing reads its manifests.
- **Bad**: `extensions/navigation` still reads every language in one package: one lexer, routes, ORMs, DI wiring and HTTP clients, with about 150 branches on the language. It breaks point 3. The fix has the same shape: a neutral `navigation` (lexer engine, linking, views) plus one recogniser package per language or framework, registered through `engine`. That is its own piece of work.
- **Bad**: the walker always walks `.github/` and `docs/` so it can keep CODEOWNERS. That is a git-hosting convention rather than a language one, and it stays in core.
