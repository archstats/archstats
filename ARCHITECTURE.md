# Architecture

## Characteristics

These drive the decisions recorded in [docs/adr](docs/adr):

- **Truthfulness**: a number in a snapshot is either right or says why it cannot be. An edge archstats could not resolve is reported as unresolved, not dropped (ADR 0019), and the same commit scans the same, byte for byte.
- **Extensibility**: a new language is an extension, and core does not change (ADR 0001, as amended by ADR 0023).
- **Interoperability**: everything a scan finds is in one SQLite file, readable by any tool that speaks SQL.
- **Approachability**: architecture should be something a junior developer can read, so metrics carry definitions and the tables say what they hold.

## How a scan runs

```
engine.Analyze(Options)
  │  Select: always-enabled extensions + the optional ones the files call for
  │  Initializer(settings) builds each extension fresh for this scan
  ▼
core.Analyze
  1. Init            each extension registers file analyzers, editors, views, definitions
  2. Walk + analyze  every file not ignored, read in parallel by every FileAnalyzer
  3. Edit files      FileResultsEditors: component linkers resolve imports to components
  4. Aggregate       core folds files into components, directories, modules and units,
                     and builds the component graph
  5. Edit results    ResultsEditors: git, code health, CI/CD, mobile, navigation
  ▼
Results ── RenderView(name) ──▶ View (columns + rows) ──▶ SQLite / CSV / JSON / table
```

Views render lazily, when an exporter asks for them, and every one reads the finished `Results`. Extensions run in the order of the catalogue in [engine/catalog.go](engine/catalog.go). No results editor reads another's output today. If one ever needs to, give extensions declared dependencies first, rather than relying on slice order.

## The model

| Grain | What it is | Where |
|---|---|---|
| File | a path the walker read, with its role (source, test, config…), system kind, third-party and generated flags | `core/file` |
| Component | where the code says it lives: a package, namespace or directory | `core/component` |
| Module | what the project builds and publishes, read from its manifests | `core/module` |
| Unit | a named thing: type, function, module-level constant; folded across files | `core/unit` |
| Snippet | an observation: text of a type at a position, with no identity | `core/file` |

Edges are typed (ADR 0018), and run between components and between units. Core owns this model and the readers that build it, including the ecosystem manifest readers, because every extension depends on them before it can run (ADR 0023). Languages, metrics, history and views live in `extensions/`.

## Extensions

An extension implements `core.Extension` and registers what it contributes in `Init`. A front end sees it as a `plugin.Descriptor`:
- its name;
- the arguments it takes, with defaults;
- a discovery trigger;
- an `Initializer` that reads its arguments from `plugin.Settings`.

The CLI passes its flag set as the settings, and anything else passes `plugin.Values`. Extensions never import `cmd/`.

Extensions keep per-scan state, so each scan builds fresh instances.

## The snapshot is a contract

The desktop app, `archstats assert`, reports and Ask all query snapshot tables by name, so the schema is a public interface:

- Views declare their columns. `View.Check` reports a row value with no column, which is never exported, and a value of the wrong type.
- `e2eTest/testdata/snapshot_schema.golden` records every table and column of a fixture scan. Regenerate it with `UPDATE_GOLDEN=1` and review the diff.
- Anything that changes what a scan produces for the same source bumps `core.AnalysisRevision`, so readers can flag snapshots older than the engine.
- Scans are deterministic:
  - graph ids follow component names;
  - randomised algorithms are seeded;
  - ties are broken by name;
  - floats keep ten significant digits.

  `TestSnapshot_SameTreeSameSnapshot` holds this.

[DESCRIPTION.md](DESCRIPTION.md) documents every table and column.

## Packages

| Package | Role |
|---|---|
| `main.go`, `cmd/` | the CLI: flags, `view`, `export`, `assert`, `definitions` |
| `engine` | the composition root: which extensions exist, which a workspace needs, run a scan |
| `core` | the pipeline, `Results`, views and the shared model |
| `core/plugin` | how an extension is described and configured |
| `extensions/treesitter/*` | one language pack per language, sharing `treesitter/common` |
| `extensions/components`, `declbased` | component resolution, coupling, cycles, centrality |
| `extensions/git`, `matrix` | history, co-change, authorship |
| `extensions/codesmells` | code health and hotspots |
| `extensions/cicd`, `mobile`, `navigation` | how the workspace ships, what a mobile app declares, what an architect navigates by |
| `extensions/rules` | architecture rules scoped by ecosystem |
| `e2eTest` | end-to-end tests on fixtures and real projects |
