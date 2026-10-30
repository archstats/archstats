package core

// AnalysisRevision numbers what a scan means, not what the program is called.
//
// A snapshot is immutable, so a fix to the analysis never reaches the scans
// taken before it: an older snapshot keeps reporting the Go internal/ rule as
// kept on a Java codebase, or 22 modules in a PHP one, long after the engine
// stopped saying so. Readers compare the revision a snapshot was written with
// against this one and say when the snapshot predates what the engine knows.
//
// Bump it whenever a change alters what a scan produces for the same source:
// a new or corrected edge, count, rule verdict or classification. Snapshots
// written before the revision existed read as 0.
//
//	1  2026-09-23  rules scoped by ecosystem (applies_when kind); PHP read by
//	               tree-sitter; git identities merged; Go import resolution;
//	               cycles counted once per component; third-party, generated
//	               and non-code files (translations, stylesheets) get no
//	               health reading; shortest_path_length counts hops, not
//	               nodes; commits touching more than 100 files stay in
//	               git_commits but are left out of co-change.
//	2  2026-09-24  history follows moved files; time windows count back from
//	               the scanned commit; co-change no longer credits a component
//	               with commits it never had; snapshots record the commit,
//	               branch and uncommitted files they read, and what the walker
//	               left out; files carry a role; health keeps its deductions;
//	               days since last change; metric categories.
//	3  2026-09-24  smaller snapshots (file_matrix keeps co-changed pairs,
//	               directory co-change from two shared commits, Java class
//	               reachability opt-in); scan-level ignore patterns apply to
//	               files and their history; file-grain shared commits; an
//	               ambiguous dynamic lookup resolves the same way every scan.
//	4  2026-09-25  smaller snapshots again: report_id and timestamp stored as
//	               column defaults; git_commits a view over git_commit_info
//	               and git_commit_files; file_matrix opt-in (--file-matrix);
//	               component_matrix one row per co-changed pair;
//	               component_connections_indirect records next_hop, not the
//	               path text; Java counts no longer duplicated as snippets.
//	5  2026-09-27  files carry a system_kind (build, lockfile, ci, container,
//	               deploy, infra, config); file co-change counts those files
//	               too; deployables: what the workspace builds and ships, what
//	               goes into each, how they are built, where they run, what
//	               they talk to and what technology they carry.
//	6  2026-09-28  indentation read per file: the project's Prettier config
//	               or .editorconfig, else the file's own indentation. A
//	               two-space file was read at four spaces a level and came
//	               out half as deep.
//	7  2026-09-28  mobile: Swift, Objective-C and Dart read (units, markers,
//	               edges resolved per target or library); SwiftPM, Xcode and
//	               pubspec modules; Gradle modules named by project path, with
//	               type-safe accessors read, a module type and convention
//	               plugins followed; Kotlin annotations on the declaration
//	               they are written on, keyed by simple name, expect/actual as
//	               keywords; app_declarations from Android manifests,
//	               Info.plist and entitlements, with manifest markers on the
//	               classes they name; mobile apps as deployables with
//	               platform and stack.
//	8  2026-09-29  .vue and .svelte components are read: their script blocks
//	               give imports and units, the component is a unit named for
//	               its file, and a component named in the template counts as
//	               used; an import spelling out its extension (`./Panel.vue`,
//	               `./reader.js`) resolves to the module.
//	9  2026-09-29  GitHub Actions of every kind are pipelines: workflows,
//	               reusable workflows, and composite, Docker and JavaScript
//	               actions (pipelines.kind); a workflow is credited with what
//	               the local workflows and actions it uses do
//	               (pipelines.calls); GitHub releases count as publishing.
//	               Wails, Tauri and Electron apps are desktop deployables.
//	10 2026-09-29  framework roles read end to end, every pack: a constant
//	               made by defineStore, createSlice, createApi,
//	               createAsyncThunk or createSelector is a unit, and a
//	               forwardRef/memo-wrapped function one; functions nested in
//	               a function are owned by it; JS/TS heritage, type aliases
//	               and enums recorded, member decorators kept off the next
//	               class; Java annotations read qualified and put on the
//	               innermost type, nested types are units; Kotlin extension
//	               receivers and class keywords; Python superclasses and
//	               decorators on their own definition; PHP method attributes
//	               through use aliases; C# attributes on the type they are
//	               written in, records and minimal APIs; Go generic receivers
//	               and versioned import paths; Swift, Objective-C and Dart
//	               extensions, nested names and part files resolved; Android,
//	               Kotlin Multiplatform, Xcode and Dart test paths are tests.
//	11 2026-10-02  code health read from functions: complex code (lines in
//	               functions over cognitive 15), coupling (imports) and size
//	               (code lines); indentation deductions gone, kept only for
//	               languages no pack parses; markup, data and scripts get no
//	               health; components weigh files by code lines and name
//	               their least healthy file; a functions view.
//	12 2026-10-02  what each complex function's cognitive complexity is made
//	               of, construct by construct, in complexity_increments.
//	13 2026-10-02  what an architect navigates by: entry_points (routes,
//	               pages, consumers, schedules, commands, mains), data_entities
//	               and data_access (what is stored and who reads or writes
//	               it), unit_supertypes and bindings (what implements what,
//	               and where a container wires it), docs and doc_links (the
//	               written intent and the code it is about); units carry
//	               their line, signature, page rank and users; functions
//	               their signature.
//	14 2026-10-03  Go imports resolve through the module paths in go.mod:
//	               a third-party package no longer lands on a local directory
//	               whose name ends the import path, and a Go import only ever
//	               resolves to a directory of Go source.
//	15 2026-10-03  a file a loaded language reads as source is no longer
//	               dropped as binary for holding a NUL byte (TypeScript that
//	               joins keys with "\x00"); only a large share of non-text
//	               bytes excludes it. skipped_files lists every path the
//	               walker left out: ignored, binary, unreadable or failed. A
//	               directory an ignore rule excludes itself (node_modules) is
//	               never entered, whatever negations the file has.
//	16 2026-10-29  deployables become CI/CD: pipelines read the way they
//	               run (pipeline_triggers, pipeline_jobs with needs and depth,
//	               pipeline_steps, pipeline_handoffs, pipeline_outputs,
//	               pipeline_unread) for GitHub Actions, GitLab, Jenkins
//	               (scripted and shared libraries too), Rancher (CLI deploys,
//	               Fleet, Rancher Pipelines), Azure, Bitbucket and CircleCI;
//	               artifacts, artifact_contents and artifact_components
//	               replace deployables; dockerfiles and dockerfile_stages;
//	               cicd_flags; the runtime links, environments, config
//	               values and dependencies tables are gone.
//	17 2026-10-29  the same commit scans the same, byte for byte: component
//	               ids follow names, so graph metrics meet nodes in one
//	               order; communities seeded; page rank and HITS converged
//	               to 1e-12 (page rank moved in its fourth digit); cycles
//	               numbered and furthest paths chosen by name; partial
//	               classes merged in file order; floats kept to ten
//	               significant digits. summary holds numbers only (the
//	               version is engine_version in _snapshot); an empty
//	               component_communities keeps its columns.
//	18 2026-10-29  HTTP and outbound: entry_points gain id, template, auth
//	               and documented; api_operations and api_findings read
//	               OpenAPI 3 / Swagger 2 contracts and the NLGov API design
//	               rules; outbound_calls, outbound_systems and
//	               config_entries say what the code calls, where it is
//	               configured per environment and what that resolves to;
//	               calls, route_reach and route_targets trace every entry
//	               point member by member to the tables and calls it
//	               reaches.
//	19 2026-10-30  cicd_flags reports only what the files show is a risk:
//	               a port is not a URL password, a localhost check is not
//	               TLS turned off, placeholders are not secrets; a workflow
//	               only ever called, one kept outside .github/workflows, a
//	               bot-only pull_request_target, a deploy in a workflow
//	               outside the workspace, Fleet targets, a library loaded
//	               without a ref, dev Dockerfiles, USER set in an earlier
//	               stage or by a :nonroot base, scratch export stages,
//	               ADD --checksum and a .dockerignore at the build context
//	               raise nothing; dns/v1-style refs are versions; docker
//	               push needs an image. Flags, handoffs and deploys name
//	               the file their line is in (GitLab includes and
//	               !reference), and a flag is reported once.
const AnalysisRevision = 19
