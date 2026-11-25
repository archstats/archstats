# Building and changing extensions

The root `AGENTS.md` holds the invariants; this file covers how an extension is built.

## Exemplars

- `extensions/lines/extension.go`: the smallest whole extension. A `FileAnalyzer` that returns one stat per file, a metric definition embedded from `extensions/lines/definitions`, and table tests in `extensions/lines/extension_test.go`.
- `extensions/codesmells/extension.go`: a `ResultsEditor` that reads the finished results. It registers stat accumulators (how a file metric folds into components and directories), and its views live in `extensions/codesmells/functions_view.go`, each with a test beside it.

## Shape

- `Extension()` returns a fresh instance; extensions keep per-scan state, so `engine` builds new ones for every scan. Wire it in `engine/catalog.go` with a `plugin.Descriptor`.
- `Init(core.Analyzer)` registers everything: definitions, file analyzers, editors, accumulators, views. Registering after `Init` is a programmer error the analyzer refuses.
- Arguments come from `plugin.Settings` (`core/plugin`), never cobra or `os.Args`. Return errors; no `panic`, `os.Exit` or `log.Fatal` (forbidigo).
- A language pack registers its ecosystem in `Init` (module readers, conventions, import resolver). A domain extension stays neutral and reads ecosystems through plug-ins under its own folder: `extensions/cicd/buildtool`, `extensions/navigation/recogniser`.

## Metrics and views

- Metric keys are lower snake case, segments joined by a double underscore: `<family>__<measure>[__<qualifier>]`, e.g. `complexity__lines`, `complexity__indentation__max`, `codesmells__hotspot_score`. The key is the snapshot column name, so it is public once shipped.
- Every metric has a YAML definition under the extension's `definitions/` (`id`, `name`, `short_description`, `long_description`, `category`), embedded with `go:embed` and added in `Init`. It becomes a row of `_metric_definitions`.
- A view declares its columns (`core.View`); `View.Check` reports a value with no column or of the wrong type. A new table needs a section in `DESCRIPTION.md` (`TestDescriptionNamesEveryTable`).
- Emit facts with provenance (component, file, line), not display decisions.

## Determinism

- Output is a function of the tree: no `time.Now`, no unseeded randomness, no map iteration order in rows. Sort, and break ties by name.
- Seed randomised algorithms explicitly (`rand.NewSource(1)`, as `extensions/components` does for communities).
- Floats keep ten significant digits.

## Testing

- Unit-test the analyzer or view on in-memory files, as the exemplars do. A fixture under the extension's `testdata/` is for real manifests or source.
- A change to what a scan writes runs `go test ./e2eTest -run 'TestSnapshotSchema|TestScanDigest'`. A schema move means regenerating `e2eTest/testdata/snapshot_schema.golden`. A row move means bumping `core.AnalysisRevision`. Both are in the root `AGENTS.md`.
- New parsing code gets a seed in `engine/testdata/fuzz/` when it fixes a crash `FuzzScan` found.
- Done means `scripts/verify-harness.sh` passes.
