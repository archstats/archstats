# 0007. Component as a First-Class Concept

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

Archstats aims to analyze software architecture, particularly package-level coupling, abstractness, and instability. Different languages and developers model "components" differently—some use explicit declaration statements (like `package` in Java or `namespace` in C#), while others structure components based entirely on filesystem directory layouts. We need a way to support both models cleanly.

## Decision Drivers

- **Flexibility**: Must adapt to multiple languages and team-specific component definition conventions.
- **Accuracy**: Metrics (like Robert C. Martin's Instability/Abstractness) must represent the actual logical boundaries of the system.
- **Independence**: Metric calculation logic should not care how a component was declared.

## Considered Options

- **Directory-Only Boundary**: Treat every directory as a component. Simple but inaccurate for languages with explicit declarations or multi-package directories.
- **Declaration-Only Boundary**: Rely strictly on in-file declarations. Fails for languages without strong built-in module/namespace systems (e.g. older JavaScript/C files).
- **First-Class Component Concept with Pluggable Resolvers**: Introduce a generic component resolution system.

## Decision Outcome

Chosen option: **First-Class Component Concept with Pluggable Resolvers** (implemented in `extensions/treesitter/common/decl_based_components.go` and `extensions/components/dirbased/`).

We treat components as logical groupings of files. We provide:
1. **Declaration-based component resolution**: Files are grouped into components based on snippets (e.g. `component:declaration` snippets like Java's `package com.example`).
2. **Directory-based component resolution**: Files are grouped into components based on their containing directory paths if no explicit declaration is found.

### Consequences

- **Good**: Supports highly realistic codebase analysis for modern multi-paradigm systems.
- **Good**: Metrics like afferent and efferent coupling are computed uniformly regardless of whether components are directory-based or declared.
- **Bad**: If a codebase uses mixed styles, component boundaries could become overlapping or ambiguous, requiring careful configuration of the analyzer rules.
