---
layout: home
title: Archstats
---

# Archstats

Archstats reads a codebase and writes its architecture down: the components and modules it is made of, the types and functions inside them, every dependency between them, how they changed together in git, how complex and how healthy they are, and how the workspace is built and shipped.

<StatStrip />

All of it goes into one SQLite file, the **snapshot**. You read it in one of three ways:

<WaysIn />

New here? Start with [Install](install.md), then [Getting started](desktop/index.md) for the app or [Commands](cli.md) for the CLI. [Snapshot tables](snapshot.md) describes every table and column for your own SQL.

```shell
archstats -f path/to/project export sqlite snapshot.db
sqlite3 snapshot.db "SELECT name, modularity__instability FROM components ORDER BY 2 DESC LIMIT 10"
```

## What it reads

| Languages | Read with |
|---|---|
| Java, Kotlin, C#, Go, Python, PHP, JavaScript, TypeScript (with Vue and Svelte), Swift, Objective-C, Dart | tree-sitter: units, imports resolved to components and modules, cognitive complexity per function |
| Scala | regular expressions: packages and imports |

Modules come from the project's own manifests: go.mod, Maven, Gradle, npm workspaces, Composer, .csproj, SwiftPM, Xcode and pubspec. CI/CD comes from GitHub Actions, GitLab, Jenkins, Azure, Bitbucket, CircleCI and Rancher pipelines, and from Dockerfiles. [Ecosystems](ecosystems.md) says what each ecosystem looks like and why it is read the way it is.

## Where things are

- Releases and their notes: [Releases](releases.md), and on GitHub for the [engine and CLI](https://github.com/archstats/archstats/releases) and the [desktop app](https://github.com/archstats/archstats-ui/releases).
- What every metric means: [Metrics](metrics.md).
- Why the engine is built the way it is: [Decisions](adr/README.md).
- Bugs and questions: [GitHub issues](https://github.com/archstats/archstats-ui/issues) for the app, [archstats issues](https://github.com/archstats/archstats/issues) for the engine and CLI.
