# 0010. Stats Accumulator Pattern

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

When analyzing a codebase, raw metrics (e.g. lines of code, class counts, complexity) are gathered at the individual file/snippet level. These metrics must then be rolled up and aggregated at multiple higher logical boundaries (directories, modules, components). Hardcoding how each metric merges or aggregates makes the system rigid.

## Decision Drivers

- **Extensibility**: Support adding brand new metric types with custom aggregation rules (e.g. string lists vs sum integers).
- **Correctness**: Aggregated statistics must accurately reflect underlying elements (e.g., merging average vs merging sum).
- **Generality**: Keep the core aggregation pipeline abstract.

## Considered Options

- **Hardcoded Struct Aggregation**: Maintain a large struct with predefined fields and explicit summation logic in the core.
- **Extensible Accumulator Pattern**: Expose generic key-value stat maps where extensions define custom `StatAccumulatorFunction` methods to handle how values combine.

## Decision Outcome

Chosen option: **Extensible Accumulator Pattern** (defined in `core/stats/stat_accumulator.go`).

We represent statistics as arbitrary string keys mapped to numbers or structured data. Extensions register an accumulator function:
```go
type StatAccumulatorFunction func(left, right interface{}) interface{}
```
For example, lines of code use a simple sum accumulator, while file lists use a union accumulator. The core uses these registry functions during the roll-up phases.

### Consequences

- **Good**: High flexibility. Any extension can introduce a new custom metric (e.g., cyclic dependency count) and dictate exactly how it aggregates up from file level to component level.
- **Bad**: Potential for runtime type casting panic/errors if extensions register conflicting keys or incompatible types.
