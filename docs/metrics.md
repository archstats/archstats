# Archstats Metric Reference

This document lists all metric definitions registered in `archstats`.

## Code Scene Style Code Smells

### `codesmells__bumpy_road` (Bumpy Road)

**Description:** Count of files with complex nesting structures.

**Details:**
Indicates if a file has complex nesting structures (e.g. max indentation > 4 and avg indentation > 1.5), representing potential read and maintainability issues.

---

### `codesmells__code_health` (Code Health)

**Description:** A rating from 1 to 10 of code health.

**Details:**
A rating from 1 to 10 of code health, computed by subtracting points for size (LOC > 500), complex nesting (max indentation > 4), and deep average nesting (avg indentation > 1.5).

---

### `codesmells__hotspot_score` (Hotspot Score)

**Description:** An index between 0 and 100 showing hotspot risk.

**Details:**
An index between 0 and 100 showing hotspot risk. Highly modified files (high Git commits) that also have high size (LOC) are calculated as major hotspots.

---

### `codesmells__static_complexity_score` (Static Complexity Score)

**Description:** Static complexity metric (lines * nesting).

**Details:**
Calculated purely from static file analysis by multiplying file size (LOC) by maximum nesting indentation level. Used as a hotspot fallback when git historical metrics are absent.

---

## Dependency Graph & Cycles

### `cycles__short__avg` (Short Cycle Average Size)

**Description:** Average length of shortest cycles this component participates in.

**Details:**
The mean number of components in the shortest cycles that include this component. Smaller average cycle sizes indicate tight local coupling, while larger cycles suggest broader circular dependency chains.

---

### `cycles__short__count` (Short Cycle Count)

**Description:** Number of shortest dependency cycles this component participates in.

**Details:**
Counts the number of shortest (minimum-length) cycles that include this component in the dependency graph. Cycles indicate circular dependencies, which can make the system harder to understand, test, and deploy independently.

A component with many short cycles is tightly entangled with its neighbors.

---

### `cycles__short__max` (Short Cycle Maximum Size)

**Description:** Length of the longest shortest cycle this component participates in.

**Details:**
The maximum number of components in any shortest cycle that includes this component. A high max cycle size indicates involvement in a broad circular dependency chain spanning many components.

---

### `graph__betweenness` (Betweenness Centrality)

**Description:** How often a component lies on the shortest path between other components.

**Details:**
Betweenness centrality measures the fraction of shortest paths between all pairs of components that pass through a given component. Components with high betweenness act as bridges or bottlenecks in the dependency graph.

Removing or modifying a high-betweenness component is likely to disrupt communication between many other parts of the system.

---

### `graph__farness_centrality` (Farness Centrality)

**Description:** Sum of shortest path distances from this component to all other components.

**Details:**
Farness is the sum of shortest path lengths from a component to every other reachable component in the dependency graph. It is the inverse concept of closeness centrality.

Lower farness means the component is more central; higher farness means it is more peripheral.

---

### `graph__harmonic_centrality` (Harmonic Centrality)

**Description:** Average inverse distance to all other components in the dependency graph.

**Details:**
Harmonic centrality is defined as the sum of reciprocals of shortest path distances from a component to all other components. Unlike closeness centrality, it handles disconnected graphs gracefully by treating unreachable nodes as having infinite distance (reciprocal = 0).

High harmonic centrality indicates a component that can reach most other components in few hops.

---

### `graph__hits__authority_score` (Authority Score (HITS))

**Description:** How much a component is depended upon by important hubs.

**Details:**
Part of the HITS (Hyperlink-Induced Topic Search) algorithm. A good authority is a component that is depended upon by many good hubs. In a dependency graph, a high authority score indicates a foundational component that provides core functionality relied upon by orchestrating modules.

---

### `graph__hits__hub_score` (Hub Score (HITS))

**Description:** How well a component connects to important authorities.

**Details:**
Part of the HITS (Hyperlink-Induced Topic Search) algorithm. A good hub is a component that depends on many good authorities. In a dependency graph, a high hub score indicates a component that aggregates or orchestrates many important dependencies.

---

### `graph__page_rank` (PageRank)

**Description:** Importance score based on the dependency graph structure.

**Details:**
PageRank assigns a score to each component based on the number and importance of components that depend on it. A component depended upon by many important components receives a higher score.

In a dependency graph, high PageRank indicates a core component that is critical to the system's architecture.

---

### `graph__residual_closeness` (Residual Closeness)

**Description:** How close a component remains to the rest of the graph if it were removed.

**Details:**
Residual closeness measures a component's centrality by evaluating how much the overall closeness of the graph would decrease if the component were removed. It captures the structural importance of a component beyond simple distance measures.

A high residual closeness indicates that the component is a critical connector whose removal would significantly increase distances between other components.

---

## Git History & Churn

### `git__additions__total` (Addition Count)

**Description:** Total number of lines added to this file or component across all commits.

**Details:**
Sums the number of added lines across all commits that touched a file. At the component level, additions from all files are summed.

Combined with deletion count, this reveals the churn profile of a file: high additions with low deletions suggest growth, while balanced additions and deletions suggest refactoring.

---

### `git__age_in_days` (Age in Days)

**Description:** Number of days since the file was last modified in git.

**Details:**
Measures the age of a file based on the most recent commit that touched it. Higher values indicate stale or stable code. When combined with other metrics like commit count and code health, age helps identify dormant hotspots.

---

### `git__authors__total` (Author Count)

**Description:** Number of unique authors who modified this file or component.

**Details:**
Counts the number of distinct commit authors (by email) who have modified a file. At the component level, unique authors across all files are counted.

A high author count may indicate shared ownership or high-traffic code that warrants extra review attention.

---

### `git__commits__total` (Commit Count)

**Description:** Total number of unique commits that modified this file or component.

**Details:**
Counts the number of distinct git commits that touched a file. At the component level, this counts unique commits across all files in the component (a single commit touching multiple files is counted once).

High commit counts indicate frequently changed code, which is a key input for hotspot analysis.

---

### `git__deletions__total` (Deletion Count)

**Description:** Total number of lines deleted from this file or component across all commits.

**Details:**
Sums the number of deleted lines across all commits that touched a file. At the component level, deletions from all files are summed.

High deletion counts alongside high addition counts indicate active refactoring or rewrites.

---

### `git__repository` (Git Repository)

**Description:** The git repository root path relative to the analysis root.

**Details:**
Identifies which git repository a file belongs to. In monorepo setups with multiple nested .git directories, this distinguishes which repository root owns each file.

---

### `git__unique_file_changes__total` (Unique File Changes)

**Description:** Number of unique files changed in commits that touched this file or component.

**Details:**
Counts the number of distinct files that were modified alongside this file across all its commits. At the component level, this counts unique files changed in commits that touched any file in the component.

This metric reveals co-change patterns: a high count suggests the file is coupled to many other files at the change level.

---

## Java & Spring Treesitter Specific

### `java__class__declarations` (Java Class Declarations)

**Description:** The number of class declarations found in your Java files. This measures the total number of classes defined in the codebase.

**Details:**
The total number of classes defined within the file or component.

---

### `java__field__declarations` (Java Field Declarations)

**Description:** The total number of fields (variables inside a class) declared in your Java files. This measures how much state or data your classes hold.

**Details:**
Fields are variables declared inside a class block.
A very high number of fields in a single class might suggest that the class is taking on too many responsibilities (a God Class or high state complexity).

---

### `java__import__declaration` (Java Import Declarations)

**Description:** The number of import statements in your Java files. This shows how many external or internal classes your code references.

**Details:**
Tracks individual import statements in Java files.
Used to determine imports of a file, excluding ignored packages like standard java.* libraries if configured.

---

### `java__jpa__entities` (JPA Entities)

**Description:** The number of JPA entity classes (annotated with @Entity). These classes represent tables in your database and hold database records.

**Details:**
JPA Entities are lightweight persistent domain objects. Typically associated with a table in a relational database.

---

### `java__method_declarations` (Java Method Declarations)

**Description:** The total number of methods (functions inside a class) declared in your Java files. This measures the number of actions your classes can perform.

**Details:**
The total number of methods declared in the Java source files.
A large number of method declarations indicates high behavioral size/complexity.

---

### `java__spring__beans` (Spring Beans)

**Description:** The total number of Spring-managed beans (classes annotated with @Component, @Service, @Repository, @Controller, etc.). These are the building blocks of a Spring app.

**Details:**
Sums up all stereotypic Spring component/bean annotations in the component or file.
A high count suggests heavily Spring-managed business or framework layers.

---

### `java__spring__components` (Spring Components)

**Description:** The number of basic Spring component classes (annotated with @Component). These are helper classes that Spring manages for dependency injection.

**Details:**
A generic stereotype for any Spring-managed component when no more specific stereotype (like Service, Controller, or Repository) applies.

---

### `java__spring__configurations` (Spring Configurations)

**Description:** The number of Spring configuration classes (annotated with @Configuration). These classes set up and customize the beans and settings in your application.

**Details:**
Indicates that a class declares one or more @Bean methods and may be processed by the Spring container to generate bean definitions and service requests at runtime.

---

### `java__spring__controllers` (Spring Controllers)

**Description:** The number of Spring controller classes (annotated with @Controller or @RestController). These classes handle incoming web requests and direct them to the right places.

**Details:**
Spring controller classes process HTTP requests, bind user input, and return responses (often JSON or HTML).
A high density of controllers in one area indicates a major API entry-point area.

---

### `java__spring__repositories` (Spring Repositories)

**Description:** The number of Spring repository classes (annotated with @Repository). These classes handle talk to the database to save and load your data.

**Details:**
Spring repositories encapsulate storage, retrieval, and search behavior, typically wrapping a relational database using JPA or JDBC.

---

### `java__spring__request_mappings__delete` (Spring DELETE Mappings)

**Description:** The number of DELETE request endpoints (using @DeleteMapping or @RequestMapping with DELETE). These are web entry points used to remove data.

**Details:**
HTTP DELETE endpoints are used to delete resource representations.

---

### `java__spring__request_mappings__get` (Spring GET Mappings)

**Description:** The number of GET request endpoints (using @GetMapping or @RequestMapping with GET). These are web entry points used to retrieve data.

**Details:**
HTTP GET endpoints are used to fetch resources.
A high proportion of GET requests indicates a read-heavy controller.

---

### `java__spring__request_mappings__patch` (Spring PATCH Mappings)

**Description:** The number of PATCH request endpoints (using @PatchMapping or @RequestMapping with PATCH). These are web entry points used to make partial updates to data.

**Details:**
HTTP PATCH endpoints are used to apply partial modifications to a resource.

---

### `java__spring__request_mappings__post` (Spring POST Mappings)

**Description:** The number of POST request endpoints (using @PostMapping or @RequestMapping with POST). These are web entry points used to create new data.

**Details:**
HTTP POST endpoints are used to submit data to be processed or to create resources.

---

### `java__spring__request_mappings__put` (Spring PUT Mappings)

**Description:** The number of PUT request endpoints (using @PutMapping or @RequestMapping with PUT). These are web entry points used to update or replace existing data.

**Details:**
HTTP PUT endpoints are used to upsert or replace resources.

---

### `java__spring__request_mappings__total` (Spring Request Mappings (Total))

**Description:** The total number of web endpoints (using @RequestMapping, @GetMapping, @PostMapping, etc.) in your application. It measures how many web entry points you have.

**Details:**
Total number of HTTP endpoints exposed by Spring MVC/WebFlux controllers.
Indicates the size of the application's external API surface.

---

### `java__spring__services` (Spring Services)

**Description:** The number of Spring service classes (annotated with @Service). These classes hold your application's business logic and rules.

**Details:**
Spring service classes orchestrate domain logic, communicate with repositories, and manage transactions.
They represent the core business layer of your application.

---

## Modularity & Component Structure

### `modularity__abstractness` (Abstractness)

**Description:** Ratio of abstract declarations to total declarations in a component.

**Details:**
Abstractness measures the proportion of abstract types (interfaces, abstract classes) relative to all declarations in a component. Range is 0 (fully concrete) to 1 (fully abstract).

Used together with instability to compute distance from the main sequence.

---

### `modularity__component__declarations` (Component Declaration)

**Description:** A mark that defines where a new component starts, such as a package header in Java/Kotlin or a namespace in C#. This helps identify different parts of your code.

**Details:**
Component declarations establish the logical boundaries of the system. For instance, package headers or namespace definitions.
They are used to map source files into logical components for coupling and architectural analysis.

---

### `modularity__component__imports` (Component Import)

**Description:** A connection that brings in code from another component. It shows when one package or namespace relies on another to work.

**Details:**
Component imports represent dependency edges between different parts of the system.
Analyzing these imports allows us to build a dependency graph and calculate coupling metrics like afferent and efferent coupling.

---

### `modularity__coupling__afferent` (Afferent Coupling)

**Description:** Number of components that depend on this component.

**Details:**
Afferent coupling (Ca) measures the number of other components that depend on this component. A high afferent coupling means many other components rely on this one, making it a responsibility hub.

Components with high afferent coupling should be stable and well-tested, as changes to them cascade to many dependents.

---

### `modularity__coupling__dependencies` (Direct Dependency Count)

**Description:** Number of components that this component directly depends on.

**Details:**
Counts the number of components that this component has a direct dependency edge pointing to in the dependency graph. Unlike efferent coupling which counts unique coupling relationships, dependencies counts all direct outgoing dependency edges.

---

### `modularity__coupling__dependents` (Direct Dependent Count)

**Description:** Number of components that directly depend on this component.

**Details:**
Counts the number of components that have a direct dependency edge pointing to this component in the dependency graph. Unlike afferent coupling which counts unique coupling relationships, dependents counts all direct incoming dependency edges.

---

### `modularity__coupling__efferent` (Efferent Coupling)

**Description:** Number of components that this component depends on.

**Details:**
Efferent coupling (Ce) measures the number of other components that this component depends on. A high efferent coupling means this component relies on many external dependencies.

Components with high efferent coupling are more likely to be affected by changes in their dependencies, making them less stable.

---

### `modularity__distance_main_sequence` (Distance from Main Sequence)

**Description:** Distance from the ideal balance of abstractness and stability: |A + I - 1|.

**Details:**
The main sequence is the ideal line where A + I = 1 (abstractness + instability = 1). The distance from this line measures how well a component balances stability with abstractness.

- D = 0: On the main sequence — ideal balance.
- High D near (0,0): Zone of Pain — stable and concrete. Hard to change.
- High D near (1,1): Zone of Uselessness — unstable and abstract. Over-engineered.

---

### `modularity__instability` (Instability)

**Description:** Ratio of efferent to total coupling: Ce / (Ce + Ca). Range: 0 (stable) to 1 (unstable).

**Details:**
Instability is calculated as Ce / (Ce + Ca), where Ce is efferent coupling and Ca is afferent coupling.

- I = 0: Maximally stable — the component has no outgoing dependencies. Changes to other components cannot affect it.
- I = 1: Maximally unstable — the component only has outgoing dependencies. It is entirely dependent on others and likely to be affected by external changes.

The Stable Dependencies Principle (SDP) states that components should depend in the direction of stability: unstable components should depend on stable ones.

---

### `modularity__types__abstract` (Abstract Types)

**Description:** The number of abstract types (like interfaces or abstract classes) in a file or component. These are templates that other classes must fill in.

**Details:**
Abstract types represent extensible points in your codebase.
A high proportion of abstract types allows components to be easily extended without modifying their core logic, promoting the Open-Closed Principle.

---

### `modularity__types__total` (Total Types)

**Description:** The total number of types (like classes, interfaces, structs, or records) declared in a file or component. It measures how many structural blocks you have.

**Details:**
Total types measures the size of a component in terms of raw structural declarations.
It acts as the denominator when calculating abstractness (abstract types / total types).

---

## Size & Indentation Complexity

### `complexity__files` (File Count)

**Description:** The total number of files in a component or directory. This is the simplest way to measure size and scope.

**Details:**
Counts all source code files matched during the analysis.
Helpful for understanding component scale and distributing resource metrics.

---

### `complexity__lines` (Line Count)

**Description:** The total number of lines in a file or component, including code, comments, and empty lines.

**Details:**
A simple and universal measure of the size of your code.
Higher line counts often indicate larger, more complex files or components.

---

