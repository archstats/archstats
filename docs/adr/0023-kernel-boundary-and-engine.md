# 0023. The Kernel Owns the Shared Model; `engine` Runs a Scan

- **Status**: Accepted; point 1 superseded by [ADR 0025](0025-core-knows-no-ecosystem.md), which moved the readers and conventions out of core
- **Date**: 2026-10-29

## Context and Problem Statement

ADR 0001 drew the microkernel line at orchestration: core walks, calls extensions and aggregates, and everything about a language or an ecosystem lives in `extensions/`. Its promise was that adding a language never touches `core/`.

The code stopped keeping that promise at revision 7, and for good reasons:

| In `core/` today | Why it is there |
|---|---|
| `core/module`: go.mod, Maven, Gradle, npm, Composer, csproj, SwiftPM, Xcode, pubspec readers (~1.5k lines) | Modules are read once and used by every language pack, the rules, CI/CD and mobile. Go import resolution (revision 14) needs the module path before any linker runs. |
| `core/file`: third-party, generated, role and system-kind classification | Aggregation drops third-party code from components and units, so the classification has to happen before extensions see the results. |
| `core/unit`: units, merging, resolution | ADR 0020: one polymorphic grain every consumer shares. |

Moving these into extensions would mean an extension that every other extension depends on, with nothing to say so but registration order.

The other half of the boundary was wrong in the opposite direction: five extensions imported `cmd/config` and cobra, because an extension read its settings from a `*cobra.Command`. The desktop app built a throwaway command full of flags to start a scan, and so did two task scripts. There was no way to run a scan without the CLI.

## Decision Drivers

- **Honest documentation**: an ADR that the code contradicts misleads the next reader more than no ADR.
- **One shared model**: files, components, modules and units mean the same thing to every extension.
- **Front ends are peers**: the CLI and the desktop app should start a scan the same way, through the same code.
- **Low churn**: CI/CD (ADR 0022) landed the same day and touches `core/module`.

## Considered Options

- **Move the ecosystem readers out of core** behind registration points (`RegisterModuleReader`, `RegisterFileClassifier`). Keeps ADR 0001 literally true, at the cost of ordering between extensions that nothing declares, and a large move under work in flight.
- **Say what the kernel is now**: core owns the shared model and the readers that build it; extensions own languages, metrics and views. Add an `engine` package as the one place a scan is assembled.

## Decision Outcome

Chosen option: **say what the kernel is now**.

1. `core/` owns the shared model (files, components, modules, units and the edges between them) and whatever must run before extensions can see it: the manifest readers in `core/module` and the file classification in `core/file`. A new *ecosystem* may touch `core/module`. A new *language* still needs only an extension.
2. Extensions depend on `core`, never on `cmd`. An extension describes itself with `core/plugin.Descriptor` and reads its arguments from `plugin.Settings`. A flag set is one, and `plugin.Values` is one built from a map with the declared defaults.
3. `engine` is the composition root: the catalogue of extensions (`AlwaysEnabled`, `Optional`), `Select` for which of them a workspace needs, and `Analyze(Options)` to run a scan. The CLI's `cmd/common` and the desktop app both call it. `cmd/config` keeps its names as aliases so older callers compile.
4. The snapshot schema is a contract. Views declare their columns, `View.Check` reports rows that do not fit them, and `e2eTest/testdata/snapshot_schema.golden` records every table and column of a fixture scan. A change to it is reviewed like a change to an API.
5. The same commit scans the same. Graph ids follow component names, randomised algorithms are seeded, ties are broken by name and floats are kept to ten significant digits. `TestSnapshot_SameTreeSameSnapshot` holds this.

### Consequences

- **Good**: ADR 0001's extensibility claim is true again for what it actually covers, languages. The desktop app can drop its cobra scaffolding. A schema change or a nondeterministic view fails in CI instead of in the app.
- **Good**: `engine.Analyze(engine.Options{Root: dir})` is a whole scan, which is what tests, task scripts and future front ends (an MCP server, a CI action) want.
- **Bad**: `core/` is larger than a kernel in the textbook sense, and an ecosystem change can touch it. That is the price of one shared model.
- **Bad**: extension order is still the order of the catalogue slices. Today no results editor reads another's output (the one cross-extension read, codesmells reading git's per-file stats, sees data written in the file phase). If one ever does, declared dependencies come before the second editor, not after.
