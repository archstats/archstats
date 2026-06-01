# 0002. Tree-Sitter for AST Parsing

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

To extract architecturally significant details from files (e.g., imports, class declarations, function definitions), `archstats` needs a reliable parsing mechanism. Using regular expressions (regex) is fragile, prone to false positives/negatives (e.g. comments, multi-line strings, complex nested syntaxes), and hard to maintain across complex languages like Java, Kotlin, and C#.

## Decision Drivers

- **Accuracy**: Parsing must handle language grammar edge cases, comments, and strings correctly.
- **Performance**: Parsing must be fast enough to run against massive codebases.
- **Maintainability**: Parsing rules should be declarative rather than complex regex-matching code.

## Considered Options

- **Regular Expression Matchers**: Fast and simple for simple patterns but highly fragile.
- **Language-Specific Official AST Parsers**: Maximum accuracy but highly complex to integrate, requiring distinct language tools/runtimes or complex wrappers.
- **Tree-Sitter (Incremental AST Parsing)**: High-performance, multi-language incremental parsers using query patterns.

## Decision Outcome

Chosen option: **Tree-Sitter (Incremental AST Parsing)** (implemented in `extensions/treesitter/`).

We use tree-sitter bindings for core languages (Java, C#, Kotlin) and specify queries (e.g. `(package_declaration)`) to extract semantic declarations and imports. A regex fallback (`extensions/regex/`) remains only for simple file types or legacy configurations.

### Consequences

- **Good**: Extremely high parsing accuracy, resilient to comment blocks, multi-line strings, and complex syntax styles. Declarative tree-sitter `.query` files are easy to read and maintain.
- **Good**: Fast parse speeds.
- **Bad**: Introduces cgo dependencies for tree-sitter libraries, which makes cross-compilation slightly more complex.
