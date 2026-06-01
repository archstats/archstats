# 0001. Microkernel Architecture

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

`archstats` needs to support analyzing codebases written in various languages, with different architectural metrics, and diverse reporting formats. Hardcoding language rules and metrics into a monolithic core would make it complex, hard to maintain, and difficult to extend.

## Decision Drivers

- **Extensibility**: Third-party or new language support and metrics must be easy to add without modifying core codebase.
- **Maintainability**: Low coupling between different parts of the analysis engine.
- **Robustness**: Core analyzer should remain simple and tested, delegate complexity to plugins.

## Considered Options

- **Monolithic Core**: All language parsers and metric calculations built directly into a single engine.
- **Microkernel (Plugin-based) Architecture**: A minimal core runtime that discovers, loads, and executes pluggable extensions via defined Go interfaces.

## Decision Outcome

Chosen option: **Microkernel (Plugin-based) Architecture** (defined in `core/config.go` with the `Extension` interface). 

The core kernel coordinates file discovery, analysis orchestration, and output generation. All specific language rules, metric collectors, filters, and custom export modules are implemented as independent extensions in `extensions/` (e.g. `extensions/lines`, `extensions/treesitter`).

### Consequences

- **Good**: Clean separation of concerns. Adding support for a new language (e.g. Kotlin) simply requires adding an extension under `extensions/` without touching `core/`.
- **Bad**: Requires defining rigid interfaces at the core layer. Changes to core interfaces may require updating all existing extensions.
