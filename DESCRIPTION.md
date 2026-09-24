# Archstats Go CLI: LLM Reference Manual & Static Analysis Catalog

This document is a self-contained, high-density reference manual for Large Language Models (LLMs) analyzing the **Archstats Go CLI** repository. It contains the exact technical architecture, directory layouts, mathematical formulations of code metrics, full SQLite DDL schemas, and pre-written SQL query recipes to allow exploratory architectural diagnostics.

---

## 📁 Codebase Directory Map & Architecture

*   **`cmd/`**: Bootstraps the Cobra CLI configuration and handles output formats.
    *   `root.go`: Registers global flags (like `--extensions` / `-e`, `--exclude-views`, `--sqlite`).
    *   `export/sqlite/sqlite.go`: The database driver. Handles dynamically checking existing table columns via `pragma_table_info`, generating `ALTER TABLE ... ADD COLUMN` statements, and bulk-inserting rows in optimum parameter-limited chunks.
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

`archstats export sqlite` writes one table per registered view, plus three
bookkeeping tables. Every view table carries `report_id` (the report name;
several reports can share one file) and `timestamp` (when it was written).
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
| `_snapshot` | (report, fact about the scan) | `report_id`, `key`, `value`. Keys: `analysis_revision` — the `core.AnalysisRevision` that wrote the file (a snapshot without it predates the stamp; read it as 0); `scanned_at` (RFC 3339); `report_id`; `extensions` — the extensions that ran, comma-separated; `git_head_commit` (full sha), `git_branch` (`detached` when detached) and `git_dirty_files` (uncommitted files, `git status --porcelain`) for a scan root that is a repository; `git_head_time` — the newest HEAD commit time across repositories; `git_based_on` — the time the `last_N_days` windows count back from; `git_max_changes_per_commit` and `git_sweeping_commits` — commits touching more files than the limit are left out of co-change, and how many were; `walker_ignored_files`, `walker_ignored_dirs` and `walker_ignored_top` (JSON) — what the walker left out. `ignore_globs` — the scan's own exclusions (`--ignore`, or a workspace's patterns), sorted, one per line; absent when there were none; `git_ignored_rows` — history rows dropped because their file matched those exclusions. Older snapshots have a `_snapshot` without `report_id`. |
| `_metric_definitions` | metric id | `id`, `name`, `short_description`, `long_description`, `category` (the family it belongs to, e.g. `Code health`; empty in older snapshots) |
| `definitions` | metric id | the same as `_metric_definitions`, as a view (kept for older readers) |
| `file_contents` | file (*opt-in*: `--store-content`) | `file`, `content` |

### Units of analysis

| Table | One row per | Non-metric columns |
|---|---|---|
| `components` | component (package, namespace or directory) | `name` |
| `files` | file | `name`, `directory`, `component`, `module`, `role` (`production`, `test`, `generated`, `third_party`, `non_code`; by precedence in that reverse order). Test files also count in `complexity__files__test` and `complexity__lines__test`. Third-party and generated files carry `complexity__files__third_party` / `complexity__files__generated` = 1 and no `codesmells__*` reading. |
| `directories` | directory | `name` |
| `git_repos` | git repository (*git*) | `name`, `git__shallow_clone`, `git__head_commit`, `git__branch`, `git__head_time`, `git__dirty_files`, `git__sweeping_commits` |
| `summary` | metric, totalled over the codebase | `name`, `value` |
| `modules` | build module a manifest declares | `name`, `kind` (`maven`, `gradle`, `dotnet`, `composer`, `node`, `go`, `django`), `directory`, `manifest`, `files`, `declared_dependencies`, `internal_dependencies`, `depends_on` |
| `units` | declared thing: type, function or module | `id`, `kind` (`type`, `function`, `module`), `name`, `component`, `module`, `owner`, `file`, `files`, `markers` |
| `unit_markers` | fact about a unit | `unit`, `kind`, `source` (`annotation`, `supertype`, …), `key`, `value` |
| `snippets` | recognised fragment of source | `content`, `file`, `component`, `snippet_type` (e.g. `component:import`, `function`), `begin_position`, `end_position` (`line:char`) |

Java files also carry `java_class` / `java_full_class`.

### Edges

| Table | One row per | Columns |
|---|---|---|
| `component_connections_direct` | (from, to, file) import between components | `from`, `to`, `kind` (`import`, `type_only`, `dynamic`), `file`, `reference_count` |
| `component_connections_indirect` | reachable pair | `from`, `to`, `shortest_path_length` (hops: a direct edge is 1), `shortest_path` (`a -> b -> c`) |
| `component_connections_furthest` | component | `component`, `furthest_component`, `furthest_component_distance`, `furthest_component_shortest_path` |
| `component_matrix` | component pair | `from`, `to`, `linguistic_similarity`, `git_co_changes`, `path_distance` |
| `file_matrix` | file pair | the same columns as `component_matrix`. Not symmetric: a pair can appear once or both ways, so normalise pairs before summing. |
| `unit_connections` | unit-to-unit reference | `from`, `to`, `via` (the import that carried it), `from_component`, `to_component`, `from_file`, `to_file` |
| `unit_uses` | (unit, module) use | `unit`, `module` |
| `unresolved_edges` | import that resolved to nothing | `from`, `names`, `file`, `line`, `reason` (`names a module this analysis did not see` or `named by an expression rather than a string`) |
| `java_class_connections_direct` | class-to-class reference (*Java*) | `from`, `to`, `file`, `reference_count` |
| `java_class_connections_indirect` | reachable class pair (*Java*) | `from`, `to`, `shortest_path_length`, `shortest_path` |

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
| `git_commits` | (commit, file) | `commit_hash`, `commit_time` (ISO-8601 with offset), `author_name`, `author_email`, `commit_message`, `file`, `component`, `repository`, `file_additions`, `file_deletions`, `path_at_commit` (the file's name in that commit; `file` is its name now), `change_kind` (`modify`, `rename`). Moves are followed: a renamed file's older rows carry its current name. A move that changed no lines is a `rename` row but counts in no commit total and no co-change. A file outside every component has an empty `component`. |
| `git_authors` | author, identities merged by email | `author_name`, `author_email`, `git__*` metrics over the whole history, including files no longer present |
| `git_component_shared_commits` | component pair | `pair_1`, `pair_2`, `shared_commits`, `percentage_of_all_commits_pair_1`, `percentage_of_all_commits_pair_2`, and the same per `__last_N_days` window. Commits touching more than 100 files are left out of co-change (they stay in `git_commits`). |
| `git_directory_shared_commits` | directory pair | as above |
| `git_component_cycles_shortest_shared_commits` | shortest cycle | `cycle`, `cycle_size`, `shared_commits`, `shared_commits__last_N_days` |

### Architecture rules

| Table | One row per | Columns |
|---|---|---|
| `rules` | rule verdict, or violating edge | `rule`, `status` (`violation`, `ok`, `not_applicable`), `from`, `to`, `kind`, `file`, `line`. A rule that held or had no opinion has one row with empty edge columns. |

---

## 🔬 Relational SQL Exploratory Analysis Recipes

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
    shortest_path AS ExecutionPath
FROM component_connections_indirect
WHERE `from` LIKE '%core%' 
  AND `to` LIKE '%database%'
ORDER BY shortest_path_length ASC;
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
