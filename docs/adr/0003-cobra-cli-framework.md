# 0003. Cobra CLI Command Framework

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

`archstats` is a CLI tool that needs subcommands (e.g. `view`, `export`), global options, typed flags, and robust validation of commands. Standard Go `flag` package is too primitive for complex subcommands, nested hierarchies, and structured output formatting.

## Decision Drivers

- **Developer Ergonomics**: CLI commands should be easy to define, read, and scale.
- **Robustness**: Support auto-generated help, tab completion, and strict flag checking.
- **Ecosystem Standard**: Align with well-known Go CLI patterns.

## Considered Options

- **Go standard `flag` library**: Low overhead but lacks support for multi-level subcommands and automatic documentation.
- **urfave/cli**: Strong framework, but Cobra has wider adoption.
- **spf13/cobra**: Industry-standard CLI library in the Go ecosystem (used by Kubernetes, Hugo, etc.).

## Decision Outcome

Chosen option: **spf13/cobra** (implemented in `cmd/root.go`, `cmd/view/view.go`, `cmd/export/`).

We structure commands around a root Cobra command with nested subcommands representing output actions (e.g. exporting to sqlite, outputting a table view).

### Consequences

- **Good**: Unified, robust CLI interface with built-in validation, shell auto-completion, and automatic help generation.
- **Good**: Easy integration with `spf13/pflag` for POSIX-compliant flags.
- **Bad**: Dependency on external third-party package, but one that is mature and highly stable.
