# 0005. Tabular View Abstraction

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

`archstats` supports rendering insights to multiple formats (tables, CSV, JSON, SQLite databases). Each analysis module (files, components, graphs) calculates different tables of data. Hardcoding output rendering for each perspective would lead to duplicate code and messy output logic.

## Decision Drivers

- **Interoperability**: Must support easy addition of new output exporters.
- **Consistency**: All insights should follow a uniform tabular layout with rows, columns, and headers.
- **De-coupling**: Analysis modules should not have any knowledge of how their data is printed or exported.

## Considered Options

- **Custom Structs per View**: Each view returns a highly typed Go struct. Exporters must write mapping logic for every custom struct type.
- **Tabular View Abstraction**: Expose a uniform `View` interface (`core/view.go`) consisting of metadata-rich Columns and dynamic Rows.

## Decision Outcome

Chosen option: **Tabular View Abstraction** (defined in `core/view.go`).

Every output perspective (e.g. "files", "components", "dependencies") is compiled into a `View`. A `View` has a list of `Columns` and a set of `Rows` (mapping column names to values). A `ViewFactory` pattern is used to dynamically construct these views from analysis results.

### Consequences

- **Good**: Unified interface for formatting results. Any exporter (e.g., CSV, SQLite, JSON) can consume any view and print it automatically without custom mapping code.
- **Good**: Clean separation of calculation logic from formatting/exporting logic.
- **Bad**: Type safety is bypassed at the cell level, as cells are represented as generic values (`interface{}`) that exporters must stringify or convert dynamically.
