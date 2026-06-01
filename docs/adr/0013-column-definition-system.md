# 0013. Semantic Column Definitions

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

`archstats` exports tabular views with diverse columns. A generic view with only column header strings (e.g., "loc", "instability") lacks semantic context. A user interface or export generator does not know if "loc" is an integer count, a float ratio, or a path. This prevents rich rendering, auto-formatting, and intelligent visualization options.

## Decision Drivers

- **User Ergonomics**: Beautiful CLI output formatting and capability for intelligent UI rendering.
- **Interoperability**: Standardized metadata schemas to describe metrics.
- **Modularity**: Extensions should define the semantic nature of the columns they introduce.

## Considered Options

- **String-Only Headers**: Standard tabular headers. Zero overhead, but lacks semantic power.
- **Definition Registry System**: Every column in a view can be registered with a typed `Definition` detailing its type, description, and semantic representation.

## Decision Outcome

Chosen option: **Definition Registry System** (defined in `core/definitions/`).

We introduce a metadata registry where columns are paired with rich `Definition` objects:
- `Type`: Int, Float, String, Path, etc.
- `Description`: Human-readable context explaining the column.
- `ShortName`: Concise header abbreviation.

Exporters and user interfaces read these definitions to format columns properly (e.g. aligning numbers, coloring instability metrics, converting path representations).

### Consequences

- **Good**: Premium rendering capabilities (e.g. automatically converting float Instability to percentage, or formatting Paths clickable).
- **Good**: Centralized source of documentation for what each metric represents.
- **Bad**: Extensions must do extra registration work when adding new columns to views.
