# Archstats engine

Go CLI and engine. Layout: `ARCHITECTURE.md`. Decisions: `docs/adr/README.md` (new ADRs through `./scripts/new-adr.sh`, see `.agents/skills/adr-management`).

## Boundary rule: core knows no ecosystem (ADR 0025)

Core is the model and the questions. Every answer that depends on a language, framework or build tool lives with that ecosystem.

- **Nothing language-, framework- or build-tool-specific goes in `core/`.** That covers file extensions, manifest names, test or generated-code conventions, import spellings, package managers' folders, and framework names. Core may hold only rules that hold in every language (`tests/`, `vendor/`, data formats), plus the vocabulary and precedence it applies to what extensions register. Naming a language in a comment as an example is fine; branching on one is not.
- **A language pack owns its ecosystem.** In `Init` it registers:
  - manifest readers (`RegisterModuleReader`);
  - file conventions (`RegisterConventions`);
  - an import resolver (`RegisterImportResolver`).

  Code that two packs share goes in a family package: `treesitter/jvm`, `treesitter/ecmascript` or `treesitter/apple`. `treesitter/common` is only for what works the same in every language.
- **A domain extension stays neutral.** cicd, navigation, mobile, rules and components may know their domain's own vocabulary: CI systems, container images, build tools' file names. Anything that parses one ecosystem's source or manifests, or branches on a language or module kind, goes in a plug-in subpackage named for that ecosystem (e.g. `extensions/cicd/buildtool/maven`). The plug-in imports a small contract package, never the extension, and `engine` wires it in.
- **Only `engine` names every language and extension.** Packs never import domain extensions, extensions never import each other's internals, and core never imports `extensions/`. `engine/boundary_test.go` checks the imports.
- **Navigation reads languages through recogniser plug-ins.** `extensions/navigation/recogniser` is the contract: a language's syntax (comments, strings, marks, how types are declared) and the hooks for entry points, data, wiring, symbols and outbound calls, plus the lexical toolkit they read with. `recogniser/<family>` (jvm, dotnet, golang, ecmascript, python, php, swift, dart, prisma) implements it; `engine.Recognisers` wires them. Navigation itself may name only data formats (XML, YAML, docs) and shared vocabulary such as HTTP client type names.

## Working here

- After a change to scan output, bump `core.AnalysisRevision` and say why in `core/revision.go`. Update `e2eTest/testdata/snapshot_schema.golden` with `UPDATE_GOLDEN=1 go test ./e2eTest -run TestSnapshotSchema` only when the schema change is intended.
- A refactor must leave scans unchanged: export snapshots of the `e2eTest/temp_testdata` checkouts with the old and new binaries and compare them table by table. Short-cycle columns and PageRank already vary between runs.
- No commits or pushes unless asked. Never add AI attribution anywhere.
