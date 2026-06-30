# Architecture Decision Records (ADRs)

Architecture Decision Records (ADRs) document key design decisions made in the development of `archstats`, including the context, drivers, choices, and consequences.

## How to Create a New ADR

To maintain consistency and automate numbering, use the provided helper script:

```bash
./scripts/new-adr.sh "Title of Your Decision"
```

This script will:
1. Locate the highest existing ADR number and increment it.
2. Create a new slug-cased Markdown file based on the template.
3. Automatically append the new ADR to this `README.md` index.

---

## Index of Architecture Decisions

| ID | Title | Status | Date |
|----|-------|--------|------|
| 0001 | [Microkernel Architecture](0001-microkernel-architecture.md) | Accepted | 2026-06-01 |
| 0002 | [Tree-Sitter for AST Parsing](0002-tree-sitter-parsing.md) | Accepted | 2026-06-01 |
| 0003 | [Cobra CLI Command Framework](0003-cobra-cli-framework.md) | Accepted | 2026-06-01 |
| 0004 | [Snippet-Based Code Analysis](0004-snippet-based-analysis.md) | Accepted | 2026-06-01 |
| 0005 | [Tabular View Abstraction](0005-view-abstraction.md) | Accepted | 2026-06-01 |
| 0006 | [Gonum for Graph Metrics](0006-gonum-for-graph-metrics.md) | Accepted | 2026-06-01 |
| 0007 | [Component as a First-Class Concept](0007-component-first-class-concept.md) | Accepted | 2026-06-01 |
| 0008 | [Concurrent Bounded File Walking](0008-concurrent-file-walking.md) | Accepted | 2026-06-01 |
| 0009 | [Decoupled Output Format Exporters](0009-multiple-output-formats.md) | Accepted | 2026-06-01 |
| 0010 | [Stats Accumulator Pattern](0010-stats-accumulator-pattern.md) | Accepted | 2026-06-01 |
| 0011 | [Git Mining as an Opt-In Extension](0011-git-analysis-extension.md) | Accepted | 2026-06-01 |
| 0012 | [Ignore Mechanism via standard Ignore Files](0012-gitignore-support.md) | Accepted | 2026-06-01 |
| 0013 | [Semantic Column Definitions](0013-column-definition-system.md) | Accepted | 2026-06-01 |
| 0014 | [Multi-Phase Analysis Pipeline](0014-multi-phase-analysis-pipeline.md) | Accepted | 2026-06-01 |
| 0015 | [Dynamic CLI Configurable Extensions](0015-cli-extension-configuration.md) | Accepted | 2026-06-01 |

| 0016 | [Filter Git Noise by Max Files Per Commit](0016-filter-git-noise-by-max-files-per-commit.md) | Proposed | 2026-06-14 |
