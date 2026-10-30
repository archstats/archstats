[![Go](https://github.com/archstats/archstats/actions/workflows/ci.yml/badge.svg)](https://github.com/archstats/archstats/actions/workflows/ci.yml)

# Archstats

Archstats reads a codebase and writes down its architecture: the components and modules it is made of, the units (types, functions) inside them, every dependency between them, how they changed together in git, how complex and how healthy they are, and how the workspace is built and shipped.

It writes all of it to one SQLite file, the **snapshot**. Everything else reads that file: the Archstats desktop app, `archstats assert`, reports, and any SQL you write yourself. The tables are documented in [DESCRIPTION.md](DESCRIPTION.md).

## Install

```shell
go install github.com/archstats/archstats@latest
```

Binaries for macOS, Linux and Windows are on the [releases page](https://github.com/archstats/archstats/releases).

## Use

Write a snapshot of a codebase:

```shell
archstats -f path/to/project export sqlite snapshot.db
```

Archstats switches on what the workspace needs by looking at its files: a language pack for each language it finds, and git history when the folder is a repository, or holds several. Name extensions with `-e` to choose them yourself.

Look at one view in the terminal:

```shell
archstats -f path/to/project view components -c name,modularity__coupling__afferent,modularity__coupling__efferent,modularity__instability --sorted-by modularity__instability
```

`archstats definitions` lists every metric and what it means. `-o csv`, `-o json` and `-o tsv` change the output format.

Hold a codebase to rules written as SQL against the snapshot:

```shell
archstats assert -r rules.yml -f path/to/project
```

```yaml
assertions:
  - name: "The domain does not depend on the web layer"
    query: |
      SELECT "from", "to", file FROM component_connections_direct
      WHERE "from" LIKE 'shop/domain%' AND "to" LIKE 'shop/web%'
    expect: 0
```

[docs/rules](docs/rules) has templates.

## What it reads

| Languages | Read with |
|---|---|
| Java, Kotlin, C#, Go, Python, PHP, JavaScript, TypeScript (with Vue and Svelte), Swift, Objective-C, Dart | tree-sitter: units, imports resolved to components and modules, cognitive complexity per function |
| Scala | regular expressions: packages and imports |

Modules come from the project's own manifests: go.mod, Maven, Gradle, npm workspaces, Composer, .csproj, SwiftPM, Xcode and pubspec. CI/CD comes from GitHub Actions, GitLab, Jenkins, Azure, Bitbucket, CircleCI and Rancher pipelines, and from Dockerfiles. [docs/ecosystems.md](docs/ecosystems.md) says what each ecosystem looks like and why it is read the way it is. [docs/metrics.md](docs/metrics.md) explains the metrics.

Files are left out the way git leaves them out: `.gitignore` and `.archstatsignore` anywhere in the tree, plus `--ignore` patterns.

## Use it as a library

```go
results, err := engine.Analyze(engine.Options{Root: "path/to/project"})
```

`engine.Options` takes ignore patterns, the extensions to run and their settings (`plugin.Values` reads them from a map, with the declared defaults for anything left out). The results render to the same views the CLI exports. [ARCHITECTURE.md](ARCHITECTURE.md) explains how a scan runs.

## Develop

```shell
go test ./...
```

The end-to-end tests in `e2eTest` clone a few real projects the first time they run. Two of them guard the snapshot itself:
- `TestSnapshotSchema` compares every table and column with `e2eTest/testdata/snapshot_schema.golden`;
- `TestSnapshot_SameTreeSameSnapshot` checks that two scans of one commit write the same rows.

A change to what a scan produces bumps `core.AnalysisRevision` ([core/revision.go](core/revision.go)), so readers can tell an older snapshot from a newer one. Decisions are recorded in [docs/adr](docs/adr).
