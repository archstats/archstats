# 0004. Snippet-Based Code Analysis

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

Different extensions need to gather arbitrary metadata from files to compile diverse metrics (such as line counts, cyclical dependencies, functions, and complexity). Storing high-level language structures directly in standard file structs creates high coupling and makes it difficult for extensions to introduce brand new types of architectural entities.

## Decision Drivers

- **Generality**: The core analyzer should not know or care about language-specific details like "Java Classes" or "C# Namespaces".
- **Flexibility**: Extensions must be able to report any specific portion of code as architecturally significant.
- **Normalization**: Standardize metric calculation over a unified intermediate model.

## Considered Options

- **Strongly-Typed AST Representation**: Core exposes complex objects (e.g. `Class`, `Interface`, `Method`) which extensions must map to. Very hard to adapt to future paradigms or untyped languages.
- **Atomic Snippets**: Represent every architecturally significant unit as a generic, metadata-rich `Snippet` (`core/file/snippet.go`).

## Decision Outcome

Chosen option: **Atomic Snippets** (defined in `core/file/snippet.go`).

A `Snippet` consists of:
- A `Type` (e.g. `component:declaration`, `function`, `type:abstract`)
- A `Value` (e.g. the component or function name)
- The source `File` context.

All language parsers parse files into generic snippets. The core analysis pipeline then aggregates these snippets (e.g., counting types to compute abstractness, parsing imports to build dependency graphs).

### Consequences

- **Good**: The core analyzer remains completely language-agnostic.
- **Good**: Extensions can define custom snippet types without modifying the core logic.
- **Bad**: Downstream processors (e.g., metric aggregators) must rely on string comparisons or query matching on the snippet types and values.
