# 0012. Ignore Mechanism via standard Ignore Files

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

Analyzing codebases requires skipping build artifacts, dependency directories (e.g. `node_modules`, `vendor`), configuration files, and temporary databases (like `.db` files). Hardcoding ignored paths in the walker is inflexible and fails to respect existing project layout setups.

## Decision Drivers

- **Developer UX**: Honor existing developer workflows (e.g., if a developer ignored something in git, they probably want it ignored in architectural statistics).
- **Control**: Support tool-specific ignore configurations.
- **Performance**: Prune ignored directories early in the directory traversal tree before reading or walking their children.

## Considered Options

- **Hardcoded Skips**: Skip a static list of common names (like `.git`). Inflexible.
- **GitIgnore and ArchstatsIgnore support**: Dynamically parse standard `.gitignore` and optional tool-specific `.archstatsignore` files during the directory walk.

## Decision Outcome

Chosen option: **GitIgnore and ArchstatsIgnore support** (implemented in `core/walker/ignore_file.go` and integrated into the bounded concurrent walker).

The walker dynamically reads `.gitignore` and `.archstatsignore` files, building pattern-based filters. If a directory is determined to be ignored, the walker prunes the entire subtree immediately to save file-system search time and prevent reading untracked files.

### Consequences

- **Good**: Safe, out-of-the-box behavior that respects the repository structure perfectly.
- **Good**: Drastic performance improvements on modern repositories by avoiding deep walking of ignored directories like `node_modules` or `target`.
- **Bad**: Pattern parsing can add slight overhead, and complex gitignore pattern matching needs precise parsing to match standard Git specifications.
