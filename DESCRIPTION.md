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

## 💾 The SQLite Data Model: Complete DDL Schemas

Below are the exact SQLite DDL tables generated by `export.go`. Use this structural reference to build correct SQL statements:

```sql
-- Core components table documenting modularity, graph, git and cycle metrics
CREATE TABLE IF NOT EXISTS `components` (
    `name` TEXT PRIMARY KEY,
    `complexity__files` INTEGER,
    `complexity__lines` INTEGER,
    `complexity__indentation__avg` REAL,
    `complexity__indentation__max` INTEGER,
    `modularity__coupling__afferent` INTEGER,
    `modularity__coupling__efferent` INTEGER,
    `modularity__abstractness` REAL,
    `modularity__instability` REAL,
    `modularity__distance_main_sequence` REAL,
    `cycles__short__count` INTEGER,
    `cycles__short__avg` REAL,
    `cycles__short__max` INTEGER,
    `graph__page_rank` REAL,
    `graph__betweenness` REAL,
    `graph__harmonic_centrality` REAL,
    `graph__farness_centrality` REAL,
    `graph__residual_closeness` REAL,
    `graph__hits__authority_score` REAL,
    `graph__hits__hub_score` REAL,
    `git__commits__total` INTEGER,
    `git__authors__total` INTEGER,
    `git__additions__total` INTEGER,
    `git__deletions__total` INTEGER,
    `git__age_in_days` INTEGER,
    `report_id` TEXT,
    `timestamp` DATE
);

-- Direct package import connections
CREATE TABLE IF NOT EXISTS `component_connections_direct` (
    `from` TEXT,                  -- Source importing component name
    `to` TEXT,                    -- Target imported component name
    `file` TEXT,                  -- Source file path causing the dependency
    `reference_count` INTEGER,    -- Number of references/imports inside that file
    `report_id` TEXT,
    `timestamp` DATE
);

-- Pre-calculated transitive dependency shortest paths
CREATE TABLE IF NOT EXISTS `component_connections_indirect` (
    `from` TEXT,
    `to` TEXT,
    `shortest_path_length` INTEGER, -- Number of network hops
    `shortest_path` TEXT,           -- Hops string representation (e.g. 'A -> B -> C')
    `report_id` TEXT,
    `timestamp` DATE
);

-- Raw registry of parsed semantic fragments
CREATE TABLE IF NOT EXISTS `snippets` (
    `content` TEXT,                -- Matched source code snippet
    `file` TEXT,                   -- Absolute or relative file path
    `component` TEXT,              -- Parent namespace component
    `snippet_type` TEXT,           -- Snippet type (e.g. 'component:import', 'function')
    `begin_position` TEXT,         -- Starting location string ('Line:Char')
    `end_position` TEXT,           -- Ending location string ('Line:Char')
    `report_id` TEXT,
    `timestamp` DATE
);

-- Column definitions lookup table
CREATE TABLE IF NOT EXISTS `_metric_definitions` (
    `id` TEXT,                     -- Column key name (e.g. 'complexity__lines')
    `name` TEXT,                   -- Nice friendly name (e.g. 'Lines of Code')
    `short_description` TEXT,
    `long_description` TEXT,
    `report_id` TEXT,
    `timestamp` DATE,
    PRIMARY KEY (id, report_id)
);
```

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
