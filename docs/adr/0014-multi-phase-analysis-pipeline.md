# 0014. Multi-Phase Analysis Pipeline

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

Analysis in `archstats` involves multiple logical steps: reading files, extracting snippets, modifying results per file, aggregating metrics at component level, and editing aggregated results. Mixing these steps together into a monolithic loop makes adding intermediate filters or modifiers (like git history additions or dynamic ignores) near impossible.

## Decision Drivers

- **Maintainability**: High separation of concerns.
- **Extensibility**: Pluggability at different phases of the lifecycle (e.g. modify file results before aggregation vs modify components after aggregation).
- **Orchestration simplicity**: Clear, deterministic execution sequence.

## Considered Options

- **Single-Loop Aggregator**: Walk, parse, and aggregate immediately in a single pass. Fast but highly rigid.
- **Multi-Phase Pipeline**: Split execution into dedicated sequential lifecycle hooks.

## Decision Outcome

Chosen option: **Multi-Phase Pipeline** (implemented in `core/analyzer.go`).

We partition analysis into distinct sequential phases:
1. **File Analyzers**: Walk files and parse them into snippets.
2. **File Results Editors**: Extensions modify individual file results (e.g., lines extension appending line counts).
3. **Aggregation**: Core rolls up file-level statistics into component-level statistics.
4. **Results Editors**: Extensions modify the final aggregated outputs (e.g., calculating Instability from aggregated coupling counts).

Extensions register hook listeners for any of these phases.

### Consequences

- **Good**: Clean, linear flow that makes writing advanced plugins extremely easy.
- **Good**: Enables complex calculations (like graph centrality or stability metrics) to execute cleanly on fully aggregated intermediate views.
- **Bad**: Requires holding parsed intermediate models (snippets and file stats) in memory before the final aggregation, increasing memory consumption slightly.
