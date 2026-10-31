# Changelog

What changed in each release of the Archstats engine and CLI. The desktop app has its own changelog in [archstats-ui](https://github.com/archstats/archstats-ui/blob/main/CHANGELOG.md).

Write a line under **Unreleased** when a change lands that a user would notice: what they can now do, or what reads differently. `scripts/release.sh` turns that section into the release's notes and adds the commit list beneath it.

## Unreleased

- `archstats --version`, and the `engine_version` every snapshot records, name the real release. Built outside the release workflow (by `go install`, or inside the desktop app) they used to say v0.3.1.
- Documentation site at https://archstats.github.io/archstats/: install, the desktop app, the CLI, rules, and every snapshot table.
- Release notes come from this changelog.

## v0.11.0 (2026-10-29)

Analysis revision 16 (was 15): snapshots taken before this release are flagged for a rescan.

### Features

- cicd: pipelines read the way they run, replacing deployables (revision 16)

## v0.10.1 (2026-10-03)

Analysis revision 15 (was 13): snapshots taken before this release are flagged for a rescan.

### Fixes

- go: resolve imports through go.mod, not directory names (revision 14)
- walker: keep source with a NUL byte, and say what was skipped (revision 15)

## v0.10.0 (2026-10-03)

Analysis revision 13 (was 10): snapshots taken before this release are flagged for a rescan.

### Features

- health: code health read from functions, imports and size (revision 11)
- health: what each complex function's complexity is made of (revision 12)
- navigation: what an architect finds their way by (revision 13)

### Performance

- scan: fit a large repository in a small machine, and scan it faster

### Other changes

- code-health: the decision behind health and the harness that tested it

## v0.9.0 (2026-10-01)

Maintenance only.

## v0.8.0 (2026-10-01)

Analysis revision 10 (was 5): snapshots taken before this release are flagged for a rescan.

### Features

- gradle: project paths, type-safe accessors and what a module builds
- mobile: what an app declares to the platform it installs on
- modules: Swift packages and Xcode targets
- swift: a Swift language pack
- indentations: read each file's indentation width (revision 6)
- objc: an Objective-C language pack, linked with Swift
- dart: a Dart language pack and pubspec modules
- mobile: expect/actual and React Native bridge markers
- deployables: mobile apps and their stack (revision 6)
- gradle: type modules through their convention plugins
- vue: read .vue and .svelte components as units (revision 8)
- deployables: read every GitHub Actions file as a pipeline, and desktop apps
- js: a constant made by defineStore, createSlice or createApi is a unit
- units: framework roles read end to end in every pack (revision 10)

### Fixes

- kotlin: annotations belong to the declaration they are written on
- swift: carry the Swift grammar so go mod tidy works

## v0.7.0 (2026-09-28)

Analysis revision 5 (was 3): snapshots taken before this release are flagged for a rescan.

### Features

- snapshot: revision 4, smaller snapshots again
- files: system_kind on every file, and co-change for build files
- deployables: what the workspace builds and ships (revision 5)

### Fixes

- components: no communities on a graph with no edges

## v0.6.0 (2026-09-24)

Analysis revision 3 (was 2): snapshots taken before this release are flagged for a rescan.

### Features

- walker: scan-level ignore patterns (--ignore)
- snapshot: revision 3, smaller snapshots and file-grain co-change

## v0.5.0 (2026-09-24)

Analysis revision 2 (was 1): snapshots taken before this release are flagged for a rescan.

### Features

- snapshot: record the commit a scan read, and key facts by report
- git: time windows count back from the scanned commit
- git: history follows a moved file
- health: keep the inputs behind every health score
- definitions: export each metric's category
- walker: say what a scan left out
- files: give every file a role, and fail on a definition that does not load
- git: days since last change, and analysis revision 2

### Fixes

- git: a component without commits shares none

## v0.4.0 (2026-09-24)

Analysis revision 1 (was 0): snapshots taken before this release are flagged for a rescan.

### Features

- units: Go, TypeScript, JavaScript and Python declare units too
- units: edges between units, in every language

### Fixes

- cli: --snippet patterns are analysed, not ignored

### Other changes

- schema: describe every exported table, and fail when one is not

## v0.3.1 (2026-09-21)

### Fixes

- git: report a repository's path the same way on Windows

## v0.3.0 (2026-09-21)

Earlier history is in the [git log](https://github.com/archstats/archstats/commits/v0.3.0).

