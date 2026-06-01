# 0006. Gonum for Graph Metrics

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

For complex codebase coupling and structure analysis, `archstats` needs to calculate network topology metrics, such as PageRank, HITS, betweenness centrality, shortest path, and cycle detection on component import graphs. Writing these graph algorithms from scratch is complex, error-prone, and inefficient.

## Decision Drivers

- **Accuracy**: Algorithms must be mathematically correct and robust.
- **Performance**: High efficiency to construct and traverse large codebase dependency trees.
- **Maintainability**: Avoid maintaining massive custom graph algorithm implementations.

## Considered Options

- **Custom Graph Library**: Write in-house cycle detection and PageRank. Extremely high maintenance overhead.
- **Gonum/Graph Package**: A mature, highly optimized, and comprehensive scientific computing library for Go.

## Decision Outcome

Chosen option: **Gonum/Graph Package** (utilized in `extensions/components/graph_metrics.go`).

We leverage `gonum.org/v1/gonum/graph` to build directed coupling graphs of codebase components. We use its built-in algorithms for:
- PageRank (`network.PageRank`)
- HITS / Hubs and Authorities (`network.HITS`)
- Cycle detection (via depth-first search/Tarjan)

### Consequences

- **Good**: Zero algorithm maintenance overhead; fast and highly optimized network metrics.
- **Good**: Leverages a highly trusted and robust Go scientific package.
- **Bad**: Adds a dependency on `gonum`, which is a moderately large mathematical library, but is acceptable given the deep domain value.
