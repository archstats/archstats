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

- After cloning, run `go generate ./extensions/treesitter/swift/grammar` once: it downloads tree-sitter-swift's generated C, which is gitignored.
- After a change to scan output, bump `core.AnalysisRevision`, say why in `core/revision.go`, name "revision N" on its CHANGELOG line, then record the fixtures' scans with `UPDATE_GOLDEN=1 go test ./e2eTest -run TestScanDigest`. `TestScanDigest` fails when scans move without a bump, and the update refuses to record them. Update `e2eTest/testdata/snapshot_schema.golden` with `UPDATE_GOLDEN=1 go test ./e2eTest -run TestSnapshotSchema` only when the schema change is intended.
- A refactor must leave scans unchanged: `TestScanDigest` passing untouched, and `scripts/scan-parity.sh <old-ref>` reporting `same` for the `e2eTest/temp_testdata` checkouts (it builds both binaries and diffs every table).
- A change a user would notice gets a line under **Unreleased** in `CHANGELOG.md`. Releases go through `scripts/release.sh` (README, "Release").
- Docs for users live in `docs/` and publish as the site: VitePress, configured in `docs/.vitepress/config.mts` (the rail's pages are listed there) with a theme built on the desktop app's DESIGN.md tokens. Preview with `cd docs && npm ci && npm run dev`. `go test .` fails on a dead link or source path in any Markdown file.
- No commits or pushes unless asked. Never add AI attribution anywhere.

## Checks that hold the rules

- `.claude/settings.json` wires agent hooks from `scripts/agent/`: each edited Go file is gofmt'd and its package vetted; `git commit`/`git push` ask first; AI attribution and `pkill -f`/`killall` are refused; before finishing with Go changes the boundary, golden, determinism and fuzz-seed tests run. `git config core.hooksPath scripts/git-hooks` adds the commit-msg check for people too.
- `go test -short ./...` is the fast suite; without `-short` it adds real projects and the scan budget (`e2eTest/budget_test.go`).
- `engine/boundary_test.go` holds ADR 0025: the import rules, and no ecosystem name as a string in `core/`.
- `engine/fuzz_test.go` (`FuzzScan`) scans one arbitrary file under every pack; a recovered panic fails it. Fuzz with `go test ./engine -run '^$' -fuzz FuzzScan -fuzztime 2m` and commit any input it writes to `engine/testdata/fuzz/` with the fix.
- `e2eTest/invariants_test.go`: graph edge cases (no NaN or infinity, nothing fails) and metamorphic checks (moved root, symlinked root, an unrelated file).
- `golangci-lint run ./...` (`.golangci.yml`): no `os.Exit`, `log.Fatal` or `panic` outside `cmd/`, no clock or global randomness in a scan; other linters report only what a change adds over main.
- `archstats.rules.yaml` is Archstats' architecture checked by Archstats in CI (`go run . assert -r archstats.rules.yaml -f .` with the ignores in its header). Its ratchets only go down.
- `scripts/check-changelog.sh origin/main`: code that ships changed without a CHANGELOG line, or the revision moved without one naming it.
