# Archstats Go CLI: LLM Reference Manual & Static Analysis Catalog

This document is a self-contained, high-density reference manual for Large Language Models (LLMs) analyzing the **Archstats Go CLI** repository. It contains the exact technical architecture, directory layouts, mathematical formulations of code metrics, full SQLite DDL schemas, and pre-written SQL query recipes to allow exploratory architectural diagnostics.

---

## 📁 Codebase Directory Map & Architecture

*   **`cmd/`**: Bootstraps the Cobra CLI configuration and handles output formats.
    *   `root.go`: Registers global flags (like `--extensions` / `-e`, `--exclude-views`, `--sqlite`).
    *   `export/sqlite/sqlite.go`: The database driver. Handles dynamically checking existing table columns via `pragma_table_info`, generating `ALTER TABLE ... ADD COLUMN` statements, and bulk-inserting rows in optimum parameter-limited chunks.
*   **`engine/`**: The composition root the CLI and the desktop app both call. `catalog.go` lists every extension (`AlwaysEnabled`, `Optional`) with its discovery trigger; `engine.Select` picks the ones a workspace needs and `engine.Analyze(engine.Options{Root: dir})` runs a whole scan.
*   **`core/plugin/`**: How an extension is described (`Descriptor`: name, arguments with defaults, discovery trigger, `Initializer`) and configured (`Settings`: the CLI's flag set, or `plugin.Values` from a map). Extensions never import `cmd/`.
*   **`core/`**: Orchestrates the static analysis pipeline.
    *   `analyzer.go`: Defines the analysis lifecycle:
        1.  Initializes extensions (e.g. Kotlin, Git, Regex).
        2.  Triggers the file-walker to load codebase files concurrently into memory.
        3.  Runs `FileAnalyzer` plugins to capture code snippets.
        4.  Executes `FileResultsEditor` editors to assign files to parent namespaces/directories.
        5.  Aggregates snippets into rows and columns (tabular `Views`).
        6.  Invokes `ResultsEditor` post-processors (like Gonum graph calculations) to compute topological properties.
    *   `walker/walker.go`: Performs thread-safe parallel file walking. Reads files using goroutines bounded by $\min(\text{NumCPU} \times 2, 32)$. Parses `.gitignore` and `.archstatsignore` files dynamically at each directory level and merges them into an inherited traversal context.
*   **`extensions/`**: Pluggable analysis engines.
    *   `regex/regex_snippets.go`: Matches raw code bytes using Go's `regexp` and named subexpression capture groups `(?P<group_name>...)`. The capture group name becomes the `Snippet.Type` (e.g., `functions`, `routes`), enabling dynamic user-defined regex rules via the CLI.
    *   `git/parse.go`: Spawns standard subprocesses to read Git logs via:
        `git log --all --numstat --no-renames --pretty=format:[-archstatscommit-]%h--%at--%an--%ae--%s--`
        Calculates cumulative and timeline additions/deletions, unique files changed, and author counts per namespace.
    *   `components/graph_metrics.go`: Models namespace imports as a directed graph using **Gonum** (`gonum.org/v1/gonum/graph`). Computes Dijkstra shortest path matrices, PageRank, HITS hubs/authorities, and path centralities.

---

## 🔬 Mathematical Formulas & Semantic Meanings of Metrics

Archstats maps classic package and graph topology metrics to assess architectural health:

### 1. Modularity & Stability Metrics (Robert C. Martin's Formulations)
*   **Afferent Coupling ($C_a$)**: The number of external components/packages that *depend on* the current component. Measures inbound dependency density. High values indicate highly stable, critical hubs.
*   **Efferent Coupling ($Ce$)**: The number of external components/packages that the current component *depends upon*. Measures outbound dependency density. High values indicate highly volatile, dependent components.
*   **Abstractness ($A$)**: Ratio of abstract types (interfaces/abstract classes) to total types in a component:
    $$A = \frac{T_{\text{abstract}}}{T_{\text{total}}}$$
*   **Instability ($I$)**: Relative susceptibility to change:
    $$I = \frac{C_e}{C_a + C_e}$$
    *   $I = 0$: Highly stable (heavily imported, difficult to change without breaking downstream clients).
    *   $I = 1$: Highly unstable (imports many things, has no dependents, easily changed).
*   **Distance from the Main Sequence ($D$)**: Measures the balance between stability and abstractness:
    $$D = | A + I - 1 |$$
    Components should ideally lie close to the Main Sequence line ($D \approx 0$).
    *   **The Zone of Pain** ($A \approx 0, I \approx 0$): Stable, rigid, and concrete. Extremely difficult to modify (e.g., core utilities without abstraction).
    *   **The Zone of Uselessness** ($A \approx 1, I \approx 1$): Highly abstract but completely unused.

### 2. Graph Centrality & Influence (Gonum Library Implementations)
*   **PageRank Influence** (`graph__page_rank`): Measures global structural import. Formulated via Gonum's Power Method iteration with a **damping factor of $0.85$** and convergence limit of **$0.00001$**:
    $$PR(u) = \frac{1-d}{N} + d \sum_{v \in B_u} \frac{PR(v)}{L(v)}$$
*   **Betweenness Centrality** (`graph__betweenness`): Measures the fraction of all shortest paths passing through a component. Calculated using Brandes' algorithm:
    $$g(v) = \sum_{s \neq v \neq t} \frac{\sigma_{st}(v)}{\sigma_{st}}$$
    Nodes with high betweenness represent bottleneck communication bridges.
*   **HITS Authority & Hub Scores**: Iterative ranking of nodes with convergence limit **$0.00001$**:
    -   *Authority Score*: Points to components with highly referenced import targets.
    -   *Hub Score*: Points to components acting as coordinators, importing many authoritative nodes.

---

## 💾 The SQLite Data Model
<!-- --8<-- [start:tables] -->

`archstats export sqlite` writes one table per registered view, plus the
bookkeeping tables below. Every view table carries `report_id` (the report name;
several reports can share one file) and `timestamp` (when it was written).
A table created by an export holds these two as column defaults rather than
on every row; they read back the same, and a report appended later writes its
own values explicitly.
Tables marked *opt-in* only appear when their extension or view is enabled;
tables marked *git* need a git checkout.

A column whose id contains `__` is a metric: `family__metric[__window]`,
e.g. `git__commits__last_90_days`. Its name and meaning are in
`_metric_definitions` and in `docs/metrics.md`. Wide tables (`components`,
`files`, `directories`, `git_repos`, `summary`) have one such column per
metric the scan produced, so their exact column set varies by language and
extensions.

### Bookkeeping

| Table | One row per | Columns |
|---|---|---|
| `_snapshot` | (report, fact about the scan) | `report_id`, `key`, `value`. Keys: `analysis_revision` — the `core.AnalysisRevision` that wrote the file (a snapshot without it predates the stamp; read it as 0); `engine_version` — the archstats version that wrote it (the `summary` table's `version` row until it moved here); `scanned_at` (RFC 3339); `report_id`; `extensions` — the extensions that ran, comma-separated; `git_head_commit` (full sha), `git_branch` (`detached` when detached) and `git_dirty_files` (uncommitted files, `git status --porcelain`) for a scan root that is a repository; `git_head_time` — the newest HEAD commit time across repositories; `git_based_on` — the time the `last_N_days` windows count back from; `git_max_changes_per_commit` and `git_sweeping_commits` — commits touching more files than the limit are left out of co-change, and how many were; `walker_ignored_files`, `walker_ignored_dirs` and `walker_ignored_top` (JSON) — what the walker left out. `ignore_globs` — the scan's own exclusions (`--ignore`, or a workspace's patterns), sorted, one per line; absent when there were none; `git_ignored_rows` — history rows dropped because their file matched those exclusions. Older snapshots have a `_snapshot` without `report_id`. |
| `_metric_definitions` | metric id | `id`, `name`, `short_description`, `long_description`, `category` (the family it belongs to, e.g. `Code health`; empty in older snapshots) |
| `definitions` | metric id | the same as `_metric_definitions`, as a view (kept for older readers) |
| `file_contents` | file (*opt-in*: `--store-content`) | `file`, `content` |
| `skipped_files` | path the walker left out | `path` (relative to the root; a directory ends in `/` and stands for everything under it, so `node_modules/` is one row), `reason`, `detail`. Reasons: `ignored` — an ignore file, a scan-level pattern or version-control metadata excluded it; `binary` — read and not text (`detail` says why: the offset of a NUL byte, or the share of non-text bytes for a file a loaded language reads as source, which a NUL alone does not exclude); `unreadable` — could not be read (`detail` is the error); `failed` — read, but analysing it failed. Absent from snapshots before revision 15. |

### Units of analysis

| Table | One row per | Non-metric columns |
|---|---|---|
| `components` | component (package, namespace or directory) | `name` |
| `files` | file | `name`, `directory`, `component`, `module`, `role` (`production`, `test`, `generated`, `third_party`, `non_code`; by precedence in that reverse order). Test files also count in `complexity__files__test` and `complexity__lines__test`. Third-party and generated files carry `complexity__files__third_party` / `complexity__files__generated` = 1 and no `codesmells__*` reading. `system_kind` says what part of building, shipping or running the software the file describes (revision 5): `build` (pom.xml, package.json, go.mod, .csproj, Makefile…), `lockfile` (package-lock.json, go.sum…), `ci` (GitHub workflows, Jenkinsfile, .gitlab-ci.yml…), `container` (Dockerfile, Containerfile), `deploy` (compose, Kubernetes manifests, Helm charts and values, kustomize, Skaffold, SAM, serverless.yml), `infra` (Terraform, CloudFormation, Terragrunt), `config` (application.yml/.properties, appsettings.json, .env), `app_manifest` (AndroidManifest.xml, Info.plist, .entitlements; revision 7); empty for every other file. YAML and JSON with an uninformative name are classified by their first 8 KB. |
| `directories` | directory | `name` |
| `git_repos` | git repository (*git*) | `name`, `git__shallow_clone`, `git__head_commit`, `git__branch`, `git__head_time`, `git__dirty_files`, `git__sweeping_commits` |
| `summary` | metric, totalled over the codebase | `name`, `value` (NUMERIC: integers and fractions; text-valued stats such as `java_class` are left out, and the engine version is `engine_version` in `_snapshot`) |
| `modules` | build module a manifest declares | `name`, `kind` (`maven`, `gradle`, `dotnet`, `composer`, `node`, `go`, `django`, and from revision 7 `swiftpm`, `xcode`, `pub`), `type` (what it builds, where the manifest says: `android-application`, `android-library`, `android-dynamic-feature`, `android-test`, `kotlin-multiplatform`, `build-logic`, `jvm-application`, `jvm-library`; `library`, `executable`, `test`, `macro`, `plugin` for SwiftPM targets; `ios-application`, `app-extension`, `framework`, `test` for Xcode targets; `flutter-app`, `flutter-package`, `dart-package`), `directory`, `manifest`, `files`, `declared_dependencies`, `internal_dependencies`, `depends_on`. A Gradle module is named by its project path (`feature:foryou:impl`) from revision 7. |
| `units` | declared thing: type, function or module | `id`, `kind` (`type`, `function`, `module`), `name`, `component`, `module`, `owner`, `file`, `files`, `markers`; from revision 13 `line` (where it is declared in `file`), `signature` (the declaration up to its body, on one line, annotations left off), `page_rank` (PageRank over the unit graph with members rolled up into their owner, scaled so the average top-level unit is 1; tests, generated and third-party code are not ranked and their uses not counted; 0 for a member), `used_by` (units that use it) and `used_by_components` (other components holding a unit that uses it) |
| `unit_markers` | fact about a unit | `unit`, `kind`, `source` (`annotation`, `supertype`, `keyword` for what a Swift, Objective-C, Dart or Kotlin declaration was declared as — `struct`, `protocol`, `mixin`, `sealed`, `expect`, `actual`…, `manifest` for what an app manifest declares the class as — `activity`, `service`, `receiver`, `provider`, `application`, `launcher`, `exported`, `deep_link`; …), `key`, `value` |
| `snippets` | recognised fragment of source | `content`, `file`, `component`, `snippet_type` (e.g. `component:import`, `function`), `begin_position`, `end_position` (`line:char`) |
| `functions` | function, method or closure a language pack measured (revision 11; vendored and generated files left out) | `file`, `component`, `name` (under its types: `OrderService.place`; empty when anonymous), `begin_line`, `end_line`, `lines`, `cognitive` (SonarSource's cognitive complexity), `nesting`, `params`, and from revision 13 `signature` (as `units.signature`). Code health's complex-code deduction counts the lines of functions with `cognitive` over 15. |
| `complexity_increments` | step of a complex function's cognitive complexity (revision 12; functions over 15 only) | `file`, `function`, `function_begin` (joins to `functions.begin_line`), `line`, `points` (1, plus the nesting for an if, loop, switch, catch or ternary), `construct` (the keyword the code spells it with: `if`, `else if`, `for`, `catch`, `when`, `guard`, `?`, `&&`…), `nesting` |

Java files also carry `java_class` / `java_full_class`.

### Edges

| Table | One row per | Columns |
|---|---|---|
| `component_connections_direct` | (from, to, file) import between components | `from`, `to`, `kind` (`import`, `type_only`, `dynamic`), `file`, `reference_count` |
| `component_connections_indirect` | reachable pair | `from`, `to`, `shortest_path_length` (hops: a direct edge is 1), `next_hop` (the first step of the alphabetically first shortest route; the row for (`next_hop`, `to`) gives the next, until `next_hop` = `to`). Snapshots before revision 4 store `shortest_path` (`a -> b -> c`) instead. |
| `component_connections_furthest` | component | `component`, `furthest_component`, `furthest_component_distance`, `furthest_component_shortest_path` |
| `component_matrix` | component pair that changed together, once (`from` < `to`) | `from`, `to`, `linguistic_similarity`, `git_co_changes`, `path_distance` (hops from `from` to `to`, -1 when it does not reach) |
| `file_matrix` | file pair (*opt-in*, `--file-matrix`) | the same columns as `component_matrix`, for pairs that changed together, both ways round. Co-change per file pair is also in `git_file_shared_commits` (thresholded) and can be counted from `git_commits`. |
| `unit_connections` | unit-to-unit reference | `from`, `to`, `via` (the import that carried it), `from_component`, `to_component`, `from_file`, `to_file` |
| `unit_uses` | (unit, module) use | `unit`, `module` |
| `unresolved_edges` | import that resolved to nothing | `from`, `names`, `file`, `line`, `reason` (`names a module this analysis did not see` or `named by an expression rather than a string`) |
| `java_class_connections_direct` | class-to-class reference (*Java*) | `from`, `to`, `file`, `reference_count` |
| `java_class_connections_indirect` | reachable class pair (*Java*, only with `--java-class-indirect`) | `from`, `to`, `shortest_path_length`, `shortest_path` |

### Structure over the graph

| Table | One row per | Columns |
|---|---|---|
| `component_cycles_shortest` | (cycle, component) | `cycle_nr`, `component`, `cycle_size`, `cycle`. The `cycle` path repeats its first component to close the loop. |
| `component_strongly_connected_groups` | component | `group`, `group_size`, `component`, and `group__*` metrics of the whole group |
| `component_communities` | component | `community_nr`, `community_size`, `component`, and `community__*` metrics |
| `component_cycles_elementary`, `component_cycles_largest` | (cycle, component) (*opt-in*) | as `component_cycles_shortest` |

### History (*git*)

| Table | One row per | Columns |
|---|---|---|
| `git_commits` | (commit, file); a view over `git_commit_info` (one row per commit: `repository`, `commit_hash`, `commit_time`, `author_name`, `author_email`, `commit_message`) and `git_commit_files` (the rest, `path_at_commit` NULL where it equals `file`). Files written before revision 4 hold it as a table. | `commit_hash`, `commit_time` (ISO-8601 with offset), `author_name`, `author_email`, `commit_message`, `file`, `component`, `repository`, `file_additions`, `file_deletions`, `path_at_commit` (the file's name in that commit; `file` is its name now), `change_kind` (`modify`, `rename`). Moves are followed: a renamed file's older rows carry its current name. A move that changed no lines is a `rename` row but counts in no commit total and no co-change. A file outside every component has an empty `component`. |
| `git_authors` | author, identities merged by email | `author_name`, `author_email`, `git__*` metrics over the whole history, including files no longer present |
| `git_component_shared_commits` | component pair | `pair_1`, `pair_2`, `shared_commits`, `percentage_of_all_commits_pair_1`, `percentage_of_all_commits_pair_2`, and the same per `__last_N_days` window. Commits touching more than 100 files are left out of co-change (they stay in `git_commits`). |
| `git_directory_shared_commits` | directory pair | as above, for pairs sharing at least 2 commits |
| `git_file_shared_commits` | file pair, once (`file_1` < `file_2`) | `file_1`, `file_2`, `shared_commits`, `percentage_of_all_commits_file_1`, `percentage_of_all_commits_file_2` (0–100). Kept only where the two share at least 3 commits and that is at least 10% of the smaller side's commits; sweeping commits and files outside the snapshot are left out. Counted over files in a component and, from revision 5, files with a `system_kind`, so a build file or pipeline that changes with the code pairs with it. |
| `git_component_cycles_shortest_shared_commits` | shortest cycle | `cycle`, `cycle_size`, `shared_commits`, `shared_commits__last_N_days` |

### How the workspace builds and ships (revision 16)

Pipelines are read the way they run: what starts them, the jobs they run and
what each waits for, the steps inside each job, what they hand work to, and
what they produce. GitHub Actions, GitLab CI, Jenkins (declarative, scripted
and shared libraries) and Rancher (CLI deploys inside any CI job, Fleet
bundles, Rancher Pipelines) are read in full; Azure Pipelines, Bitbucket
Pipelines and CircleCI into the same model. Travis, Cloud Build, Drone,
Tekton, Buildkite and Woodpecker are listed with `read` = `none`. Nothing is
run or fetched: a remote include, template, orb or library is a hand-off, and
one held in another repository of the workspace is followed to it. Every fact
carries the `file` and `line` it was read from; what a reader could not follow
is a row in `pipeline_unread`, never a silent gap. Values of secret-looking
keys are never stored, and a password or token written into a command is
masked in step text.

A *job* is a GitHub or GitLab job, a Jenkins stage, an Azure job, a Bitbucket
step, a CircleCI workflow job or a Fleet target. Systems that run in order
(GitLab stages, Jenkins stages, Bitbucket steps) write that order as `needs`.

An *artifact* is what the workspace builds: a container image, an executable
app whose image is built elsewhere (or not at all), a serverless function, a
desktop app or a mobile app. Joins say how they were made in `resolution`:
`declared`, `path`, `build_output`, `module_dependency`, `name` (the weakest),
`repository` (the only artifact in its repository) or `paths` (what a
pipeline's path filter watches). A name that fits two things is never chosen
between; it is listed in `cicd_unresolved` as `ambiguous`.

| Table | One row per | Columns |
|---|---|---|
| `pipelines` | pipeline | `id` (its file), `name`, `system` (`github_actions`, `gitlab`, `jenkins`, `rancher_fleet`, `rancher_pipelines`, `azure_pipelines`, `bitbucket`, `circleci`, or a system not read), `kind` (`workflow`, `reusable_workflow`, `action`, `template` (only included or copied by others), `child` (a GitLab child pipeline), `library` (a Jenkins shared library step, `vars/name.groovy`), `gitops` (a Fleet bundle) or `pipeline`), `repository`, `file`, `line`, `read` (`full`, `partial`, `none`), `jobs`, `steps`, `stages` (the verbs its steps reach, in order: `build`, `test`, `scan`, `package`, `publish`, `deploy`, `approve`) |
| `pipeline_unread` | region not read | `pipeline`, `from_line`, `to_line`, `reason` (`dynamic_stages` (stages built at run time from data the file does not hold), `unsupported_syntax`, `not_supported_system`), `file` |
| `pipeline_triggers` | trigger | `pipeline`, `event` (`push`, `pull_request`, `tag`, `schedule`, `manual`, `called`, `upstream`, `other`), `filter` (branches, paths, cron or rule, as written), `file`, `line` |
| `pipeline_jobs` | job | `pipeline`, `id`, `name`, `group` (GitLab stage, Jenkins parent stage, Azure stage, Bitbucket section), `depth` (the longest chain of needs before it, for layout), `needs` (job ids, comma separated), `runner` (runs-on, agent, image, pool, executor or Fleet target), `matrix` (combinations; -1 when repeated over data the file does not hold), `environment`, `gate` (`manual`, `approval`, `environment`), `condition` (as written), `file`, `line` |
| `pipeline_steps` | step | `pipeline`, `job`, `index`, `name`, `verb` (`checkout`, `setup`, `build`, `test`, `scan`, `package`, `publish`, `deploy`, `approve`, `other`), `tool` (`maven`, `gradle`, `npm`, `docker`, `helm`, `kubectl`, `rancher`…), `target` (the image, chart or manifest it acts on), `text` (its first line, at most 160 characters, secrets masked), `file`, `line` |
| `pipeline_handoffs` | use of something else | `pipeline`, `job`, `kind` (`reusable_workflow`, `action`, `shared_library`, `library_step`, `load`, `include`, `component`, `template`, `orb`, `pipe`, `child_pipeline`, `starts_pipeline`, `helm_chart`), `target`, `ref`, `pinned` (`sha`, `version`, `branch`, `none`, `implicit` (loaded by the server for every job), empty for a file in this repository), `via` (the library a step comes from), `local_pipeline` (the pipeline it resolves to inside the workspace, any repository), `file`, `line` |
| `pipeline_outputs` | thing a job produces | `pipeline`, `job`, `step`, `kind` (`image`, `build`, `package`, `release`, `deploy`), `platform` (deploys: `rancher`, `kubernetes`, `helm`, `kustomize`, `fleet`, `compose`, `cloud`, `other`), `environment`, `target`, `artifact` (when joined), `server` and `context` (Rancher: as written, variables included), `file`, `line`, `resolution` |
| `artifacts` | artifact | `id`, `name`, `kind` (`image`, `app`, `function`, `desktop_app`, `mobile_app`), `platform` (mobile and desktop), `repository`, `dockerfile`, `context`, `module`, `built_by` (`skaffold`, `jib`, `buildpacks`, `pipeline`, `compose`, `maven-docker`, `spring-boot`, `dotnet-publish`, `dockerfile`, `maven`, `gradle`, `dotnet`, `aspire`, `sam`, `serverless`, `wails`, `tauri`, `electron`, `xcode`, `flutter`, `react-native`, `expo`, or `delegated` when a pipeline hands the build to a template outside the workspace), `base` (the final stage's base image), `runtime` (`java 17`, `node 20`…), `files`, `components`, `file`, `line` |
| `artifact_contents` | (artifact, path) | `artifact`, `path`, `pattern`, `module`, `file`, `line` (the COPY, context or manifest that put it there), `resolution` |
| `artifact_components` | (artifact, component) | `artifact`, `component`, `files` |
| `dockerfiles` | Dockerfile | `file`, `repository`, `stages`, `final_base`, `final_user`, `ports`, `entrypoint`, `healthcheck` (0/1), `dockerignore` (0/1) |
| `dockerfile_stages` | FROM | `file`, `index`, `name`, `from`, `from_kind` (`image`, `stage`, `scratch`), `image`, `tag`, `digest`, `user`, `line` |
| `cicd_flags` | hygiene fact | `rule`, `subject_kind` (`pipeline`, `job`, `dockerfile`), `subject`, `detail`, `file`, `line`. Rules: `base_unpinned_tag`, `base_no_digest`, `runs_as_root`, `add_remote_url`, `secret_like_key`, `copy_all_no_dockerignore`, `handoff_unpinned`, `library_unpinned`, `remote_include_unpinned`, `pr_target_checkout_head`, `no_permissions_block`, `curl_pipe_shell`, `docker_privileged`, `tls_verify_off`, `token_literal`, `ungated_prod_deploy` (to an environment named `prod`, `production`, `prd` or `live`), `push_latest_only`. Each is a fact with a line, not a score; `docs/rules/cicd.yml` has an assert template per rule. |
| `cicd_unresolved` | reference that joined to nothing | `from`, `ref`, `file`, `line`, `reason` (`not_built_here`, `external`, `interpolated`, `ambiguous`, `not_found`, `include_not_found`, `reference_not_found`, `template_not_found`, `job_not_found`) |

Removed in revision 16: `deployables`, `deployable_contents`,
`deployable_components`, `deployable_links`, `deployable_unresolved`,
`pipeline_deployables`, `deployable_environments`,
`deployable_environment_values` and `deployable_dependencies`. ADR 0022 says
why.

### What a mobile app declares to its platform (revision 7)

| Table | One row per | Columns |
|---|---|---|
| `app_declarations` | declaration in an app manifest | `module`, `platform` (`android`, `ios`), `kind` (`activity`, `service`, `receiver`, `provider`, `application`, `permission`, `feature`, `deep_link`; `bundle_id`, `url_scheme`, `background_mode`, `usage_description`, `ats_exception`, `scene_delegate`, `entitlement`), `name` (a class resolved against the module's namespace, a permission, a key), `value` (a deep link's URL, a usage description's text, an entitlement's values), `exported` (`true`, `false`, `implied` when an intent filter exported it before Android 12 made that explicit, empty), `launcher` (1 for the MAIN/LAUNCHER activity), `unit` (the unit the declaration names, when it is in the codebase), `file`, `line` |

### What an architect navigates by (revision 13)

Read lexically from how each framework spells it -- comments blanked,
strings kept -- and never guessed from a name. Every row carries the `file`
and `line` it was read from. Test, generated and third-party files are left
out. A `unit` or `entity` column is a `units.id`, empty where nothing in the
codebase answers to the name.

| Table | One row per | Columns |
|---|---|---|
| `entry_points` | way into the code | `id` (stable within the snapshot, what other tables name a route by), `kind` (`http`, `page` for a client-side or file-routed page, `message` for a queue, topic or socket consumer, `event` for an in-process listener, `schedule`, `cli`, `main`, `job` for a task queue worker or serverless function), `method` (`GET`, `POST,PUT`…, `ANY`, `WS`; empty when not HTTP; where the framework leaves it to names or defaults, read from them: a Django class view's `get()`/`post()` or its generic base, `@require_POST`/`@api_view` (GET when it lists none), a Next.js or SvelteKit route file's exported `GET`, an ASP.NET Web API 2 action's name (POST when none fits), Azure Functions' `HttpTrigger` methods, one row per method; a Laravel resource is one row per action; `ANY` only where the framework itself takes any), `path` (the route with its class, group or import prefix joined; a constant in it kept as `{NAME}`; the topic, queue, cron, command name or, for a `main`, its directory), `framework` (`spring`, `jax-rs`, `aspnet`, `aspnet-conventional` for an MVC action routed by the `/{controller}/{action}` convention, `aspnet-minimal`, `nestjs`, `micronaut`, `express`, `fastify`, `koa`, `hono`, `django`, `flask`, `fastapi`, `gin`, `echo`, `chi`, `fiber`, `gorilla`, `net/http`, `laravel`, `symfony`, `ktor`, `next`, `nuxt`, `sveltekit`, `react-router`, `vue-router`, `angular-router`, `spring-kafka`, `spring-amqp`, `jms`, `symfony-messenger`, `celery`, `django-signals`, `django-admin`, `cobra`, `click`, `picocli`, `symfony-console`, `azure-functions`, a language for a `main`…), `template` (the path with every parameter written `{}`, `:id`, `<int:id>`, `[id]` and regex groups alike, for comparing routes, contracts and calls), `handler` (as the route names it, or the function), `unit`, `function` (a `functions.name`), `component`, `auth` (who may call it as the code says: the handler's or its class's marks — `PreAuthorize(…)`, `Authorize(Roles = …)`, `AllowAnonymous`, `UseGuards(…)`, `login_required` — else a middleware group, Django REST framework's `permission_classes`, an Express middleware with an auth-like name, ASP.NET's `RequireAuthorization(…)`, or the first Spring Security matcher whose pattern covers the route within its filter chain's `securityMatcher`; empty when nothing says), `documented` (`spec` when a contract operation matches it, `annotations` for `@Operation`-style marks, `framework` for FastAPI), `file`, `line`. File-routed pages count only when the framework's config file is in the workspace. HTTP clients (Feign, Retrofit, Refit, axios) are not entry points; they are `outbound_calls`. |
| `data_entities` | stored thing | `entity` (the unit), `name`, `table` (the table or collection when declared; empty for a default), `store` (`sql`, `document`, empty when unknown), `framework` (`jpa`, `hibernate-xml`, `doctrine`, `doctrine-xml`, `django`, `sqlalchemy`, `flask-sqlalchemy`, `eloquent`, `ef`, `typeorm`, `prisma`, `mongoose`, `sequelize`, `gorm`, `spring-data`, or `repository` for a type known as stored only because a repository of it exists), `file`, `line` |
| `data_access` | place code touches stored data | `unit`, `function`, `component`, `target` (the entity or table as named), `entity` (empty for a table no entity declares), `access` (`read`, `write`, `read_write` for a repository), `via` (`repository`, `orm`, `dbset`, `jpql` for a query naming an entity, `sql` for one naming a table), `file`, `line`. SQL is read from string literals in files that run SQL; a statement built with `+` counts each table where its literal names it. |
| `unit_supertypes` | (unit, supertype) a type declares | `unit`, `supertype` (the unit, empty when it is outside the codebase), `name` (as written) |
| `bindings` | place an implementation is bound to what is asked for | `interface` (a type, or a bean or service id), `interface_unit`, `implementation`, `implementation_unit`, `mechanism` (`bean` and `dagger` for a provider method, `component_scan` for an annotated type found by scanning, bound to each interface in the codebase it implements, `spring_xml`, `guice`, `dotnet_di`, `autofac`, `laravel`, `symfony_php`, `symfony_xml`, `symfony_alias`, `angular`, `koin`, `hilt`), `file`, `line` |
| `docs` | Markdown, AsciiDoc or reStructuredText document | `file`, `kind` (`adr`, `readme`, `architecture`, `guide` under a docs folder, `contributing`, `changelog`, `note` for the rest), `title`, `status` (an ADR's), `date`, `words` |
| `doc_links` | (document, component) it is about | `doc`, `component`, `how` (`located_in` the component's folder, `names_path` of its files or folders, `names_component`, `names_unit` for a type or function only one unit is called), `mentions` |
| `api_operations` | operation in an API contract | `spec` (the OpenAPI 3 or Swagger 2 file), `title`, `version` (`info.version`), `method`, `path`, `template`, `operation_id`, `tag` (the first), `summary`, `deprecated` (0/1), `security` (scheme names, `none` for an explicit empty requirement), `side` (`provided` when a server generator is pointed at it or a route matches, `consumed` when a client generator is, `unknown`), `entry` (the `entry_points.id` it matches, by method and template under any server base path and the application's context path; 0 when none), `line`. |
| `api_findings` | NLGov REST API design rule a contract or route breaks | `rule` (`openapi3`, `semver`, `info-contact-fields-exist`, `servers-use-https`, `include-major-version-in-uri`, `missing-version-header`, `use-problem-schema`, `http-methods`, `paths-no-trailing-slash`, `paths-kebab-case`; named after the Logius Spectral ruleset), `severity` (`must`, `should`), `subject` (`spec`, `operation`, `route`), `entry`, `spec`, `method`, `path`, `detail`, `file`, `line`. Routes are checked only when they look like an API: in a component a contract matches, or under `/api`. A finding is a fact, not a conformance verdict. |
| `outbound_calls` | place the code calls out | `id`, `kind` (`http`, `soap`, `grpc`, `graphql`, `message`, `sql`, `nosql`, `cache`, `search`, `mail`, `ldap`, `storage`, `file_transfer`), `client` (`feign`, `retrofit`, `refit`, `microprofile`, `http-exchange`, `resttemplate`, `webclient`, `restclient`, `java.net.http`, `okhttp`, `httpclient`, `restsharp`, `axios`, `fetch`, `angular-http`, `nestjs-http`, `requests`, `httpx`, `aiohttp`, `net/http`, `resty`, `guzzle`, `laravel-http`, `curl`, `openapi-generator`, `kafka`, `amqp`, `jms`, `sqs`, `celery`, `jdbc`, `ef-core`, …), `method`, `path` (with parameters as written, or the topic, queue or service), `template`, `target` (the address as written: `${partner.url}/orders/{id}`), `system` (the declared client name, the configuration key's meaningful part, the host, or `self`), `host` (production's, else the default's), `confidence` (`declared`, `resolved`, `partial` when a key or name is known but no host, `unresolved`), `resolved_via` (`literal`, `config`, `env`, `declared`, `generated`), `config_key`, `entry` (the route a call to this workspace reaches), `consumer` (the consumer of the topic or task it sends), `resilience` (`timeout`, `retry`, `circuit_breaker` from marks, call options, client registration or client configuration), `unit`, `function`, `component`, `file`, `line`. Calls are gated on the receiver being that client, by its declared type or a name no other object goes by. |
| `outbound_systems` | system the workspace reaches, per environment | `system`, `kind` (as `outbound_calls`, plus `identity` for an OAuth issuer or identity provider), `environment` (from the configuration file's name or folder, OTAP and DTAP names normalised to `dev`, `test`, `acc`, `prod`; empty for every environment), `host`, `source` (`config`, `call`, `generator`), `config_key`, `file`, `line`. |
| `config_entries` | configuration key that names a place | `key`, `value` (a URL, host or connection string with its user and password removed, or a reference such as `${PARTNER_URL:http://localhost:8081}`; no other value is ever kept), `environment`, `format` (`spring`, `dotnet`, `dotnet_xml`, `dotenv`, `compose`, `helm`, `yaml` for Kubernetes manifests, `laravel`, `django`, `node`), `file`, `line`. |
| `calls` | call one function makes to another, reached from an entry point | `from_unit`, `from_function`, `to_unit`, `to_function` (a `functions.name`, or `Unit.member` for a member with no body: a repository or client interface method), `via` (`field`, `static`, `self`, `import`, `function`, `implementation` for an interface's implementing type found through `unit_supertypes` or `bindings`), `file`, `line` (the call site). Read lexically: a receiver's declared type, an import, a static type name or a bare function name; an untyped local is not followed. |
| `route_reach` | what an entry point reaches | `entry`, `call_sites` (`resolved` plus `unknown`), `resolved`, `unknown` (calls on a receiver whose type could not be told), `units`, `functions`, `tables`, `outbound`, `depth`. Calls into libraries are counted in neither. Traces stop at depth 8 and 400 functions. |
| `route_targets` | table or outbound call an entry point reaches | `entry`, `kind` (`table`, `outbound`), `target` (a `data_access` entity or table, or an `outbound_calls.id`), `depth`. |

### Architecture rules

| Table | One row per | Columns |
|---|---|---|
| `rules` | rule verdict, or violating edge | `rule`, `status` (`violation`, `ok`, `not_applicable`), `from`, `to`, `kind`, `file`, `line`. A rule that held or had no opinion has one row with empty edge columns. |

---

<!-- --8<-- [end:tables] -->

## 🔬 Relational SQL Exploratory Analysis Recipes
<!-- --8<-- [start:recipes] -->

Below are pre-written, syntactically correct SQL queries to analyze the database. Feed these to exploratory LLMs to query custom structural diagnostics:

### 1. Identify Components in the "Zone of Pain"
Stable, rigid, concrete packages that are highly imported but have near-zero abstractness, representing severe refactoring risks:
```sql
SELECT 
    name, 
    modularity__coupling__afferent AS InboundDependents,
    modularity__coupling__efferent AS OutboundDependencies,
    ROUND(modularity__abstractness, 3) AS Abstractness,
    ROUND(modularity__instability, 3) AS Instability,
    ROUND(modularity__distance_main_sequence, 3) AS Distance
FROM components
WHERE modularity__abstractness < 0.1 
  AND modularity__instability < 0.1 
  AND modularity__coupling__afferent > 2
ORDER BY modularity__coupling__afferent DESC;
```

### 2. Spot Cognitive Nesting Hotspots with High Commit Churn
Locates packages that developers modify constantly, but which contain highly nested, complex control structures (max indentation > 4):
```sql
SELECT 
    name,
    complexity__lines AS LOC,
    complexity__indentation__max AS MaxIndentation,
    git__commits__total AS TotalCommits,
    git__authors__total AS TotalAuthors
FROM components
WHERE complexity__indentation__max >= 5 
  AND git__commits__total > 10
ORDER BY git__commits__total DESC, complexity__indentation__max DESC;
```

### 3. Track Transitive Path Dependencies between Two Specific Namespaces
Finds the shortest structural dependency paths showing how package `A` transitive-imports package `B`:
```sql
SELECT 
    `from` AS SourceComponent,
    `to` AS TargetComponent,
    shortest_path_length AS HopCount,
    next_hop AS FirstStep
FROM component_connections_indirect
WHERE `from` LIKE '%core%' 
  AND `to` LIKE '%database%'
ORDER BY shortest_path_length ASC;

-- The whole route for one pair, step by step:
WITH RECURSIVE route(step, component) AS (
    SELECT 0, 'shop.core'
    UNION ALL
    SELECT step + 1, i.next_hop
    FROM route r JOIN component_connections_indirect i
      ON i.`from` = r.component AND i.`to` = 'shop.database'
    WHERE r.component <> 'shop.database'
)
SELECT component FROM route ORDER BY step;
```

### 4. Locate Dependency Cycles and Circular Rigidity
Isolates namespaces locked inside circular import loops, preventing modular isolation and independent microservice extractions:
```sql
SELECT 
    name,
    cycles__short__count AS CyclesCount,
    ROUND(cycles__short__avg, 1) AS AvgCycleNodesSize,
    cycles__short__max AS MaxCycleNodesSize
FROM components
WHERE cycles__short__count > 0
ORDER BY cycles__short__count DESC;
```

### 5. Find Network Bottlenecks using PageRank and Betweenness Centrality
Locates architectural hubs that serve as major transition bridges, carrying high structural gravity in the codebase:
```sql
SELECT 
    name,
    ROUND(graph__page_rank, 5) AS PageRank,
    ROUND(graph__betweenness, 2) AS Betweenness,
    modularity__coupling__afferent AS DirectInbound,
    modularity__coupling__efferent AS DirectOutbound
FROM components
ORDER BY graph__page_rank DESC, graph__betweenness DESC
LIMIT 10;
```
<!-- --8<-- [end:recipes] -->
