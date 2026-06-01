# 0009. Decoupled Output Format Exporters

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

`archstats` is built to integrate with diverse developer workflows. Some users need rapid command-line visualization (tables), some need structured data for downstream scripting (JSON, CSV), and some need relational querying capabilities for deep exploration (SQLite). Decoupling these exporters from the analysis core is essential to prevent logic pollution.

## Decision Drivers

- **Interoperability**: Ease of exporting analysis results to external tools.
- **Pluggability**: Ability to write custom exporter engines without modifying the metric calculation layer.
- **Maintainability**: Low coupling between the data calculation layer and rendering/printing formatting layer.

## Considered Options

- **Direct File Writers in View Modules**: Have the view calculation code print itself. Heavy code repetition and tight coupling.
- **Decoupled Output Format Exporters**: Exporters consume a generic `View` structure and format it accordingly.

## Decision Outcome

Chosen option: **Decoupled Output Format Exporters** (implemented in `cmd/view/view.go` and `cmd/export/sqlite/`).

We provide multiple CLI output configurations:
- `table`: Human-readable formatted console printing.
- `csv` / `tsv`: Delimiter-separated files.
- `json` / `ndjson`: Machine-readable structured objects.
- `sqlite`: Fully relational database export where each view becomes a table.

All exporters run against the generic `View` model (`core/view.go`), ensuring zero coupling to specific language models or metrics.

### Consequences

- **Good**: Adding a new export target (e.g. HTML dashboard or Markdown reports) requires minimal effort and operates completely downstream of the calculation code.
- **Good**: Enables robust integration with other tools (e.g., querying dependency data using custom SQL in `sqlite`).
- **Bad**: Complex exports (like relational schema mapping for SQLite) require extra conversion layer complexity (e.g. converting `interface{}` values to database-supported primitives).
