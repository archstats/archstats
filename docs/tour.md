# Tour of the app

This page walks through Archstats Desktop one view at a time: a picture of each, what it shows, and how to read it. Read it top to bottom once, or jump to the view that answers your question.

The pictures are the app on [Apache Fineract](https://github.com/apache/fineract), a Java and Spring banking platform of 1.4 million lines, unless the caption names another codebase. Click any picture to see it full size.

## Start from a question {#questions}

<!-- capabilities:begin. Generated from archstats-ui's capability catalogue (frontend/src/features/capabilities); edit it there and run `UPDATE_DOCS=1 npx vitest run src/features/capabilities` in archstats-ui/frontend. -->

| I want to… | Start in | Then |
|---|---|---|
| Get a first read of an unfamiliar codebase | [Overview](#overview), [Units](#units) | [Connections](#connections), [Metrics](#metrics), [Libraries](#libraries), [Authors](#authors), [Ask](#ask) |
| Find where something lives, and follow a request through the code | [HTTP](#http) | [File page](#file), [Units](#units), [Ask](#ask) |
| Find out why the code is built the way it is | [Component page](#component), [Ask](#ask) | [File page](#file), [Authors](#authors) |
| Know what a change could break | [Connections](#connections) | [Component page](#component), [File page](#file), [Units](#units), [Ask](#ask) |
| Write an assessment or due-diligence report | [Overview](#overview), [Reports](#reports) | [Hotspots](#hotspots), [Cycles](#cycles), [Authors](#authors), [HTTP](#http), [CI/CD](#cicd), [Libraries](#libraries) |
| Decide which technical debt to pay first | [Hotspots](#hotspots), [Overview](#overview) | [Metrics](#metrics), [Activity](#activity), [File page](#file) |
| Find code nothing uses | [Units](#units) | [Metrics](#metrics) |
| Make the case for a change to the team or management | [Reports](#reports) | [Changes](#changes), [Hotspots](#hotspots), [Cycles](#cycles) |
| Check the code keeps its layers and module rules | [Units](#units), [Rules](#rules) | [Connections](#connections), [Lenses](#lenses), [Metrics](#metrics) |
| Fail the build when the architecture drifts | [`archstats assert`](cli.md#rules) | [Rules](#rules), [SQL console](#sql) |
| Untangle circular dependencies | [Cycles](#cycles) | [Connections](#connections), [File page](#file), [Changes](#changes) |
| Find the domains, or carve out a module or service | [Lenses](#lenses), [Connections](#connections) | [HTTP](#http), [Ask](#ask) |
| Gauge framework lock-in, or the scope of an upgrade | [Libraries](#libraries) | [Units](#units) |
| See what changed since a release or a big merge | [Changes](#changes) | [Rules](#rules), [Reports](#reports) |
| Tell whether the architecture is getting better or worse | [Over time](#changes) | [Overview](#overview) |
| Find knowledge risk, and who to ask about a part | [Authors](#authors) | [Activity](#activity), [Ask](#ask) |
| See where the team's effort goes | [Activity](#activity) | [Hotspots](#hotspots), [Authors](#authors) |
| Review an HTTP API: who may call it, what it writes, what it calls | [HTTP](#http) | [Reports](#reports) |
| Review how the workspace is built and shipped | [CI/CD](#cicd) | [Reports](#reports) |
| Answer a question no view asks | [Ask](#ask), [SQL console](#sql) | [`archstats view`](cli.md#look-at-one-table), [`archstats export`](cli.md#write-a-snapshot), [The snapshot file](snapshot.md) |

<!-- capabilities:end -->

## The whole picture

### Overview {#overview}

<Shot name="overview" title="Overview" source="Apache Fineract">
The first screen after a scan.
</Shot>

The Overview is where every snapshot opens. Across the top, one strip gives the size of the codebase: components, files, lines, code health, commits and contributors. Below it:

- **Code** splits the lines by language and by role (production, tests, third-party, not code), then gives the structure in numbers: how far a change spreads (*propagation cost*), how much of the code sits in tangles, the largest tangle and the number of dependency levels.
- **Activity** shows a year of commits as a calendar, and every month's lines added and removed since the first commit.
- **Code health** says how much of the code is healthy and how much of the change effort goes into code that is not, then lists the lowest-scoring components and files.

The panels link into the views that explain them, so the Overview is also the way into the rest of the app.

### Ask {#ask}

Ask is a chat that answers from the snapshot, and draws the figure or table behind each answer so you can check it. It is off until you turn AI on in **Settings**, can run on a model on your own machine, and stays off when your organisation's policy says so.

## What it serves and ships

### HTTP {#http}

<Shot name="http" title="HTTP" source="Apache Fineract">
817 endpoints in 156 classes, and how they reach down through the layers.
</Shot>

Every endpoint the code declares, under the class that answers it, with its method and path. The picture on the right follows all of them down: from the endpoints, through controllers, services and repositories, to the tables at the bottom and the other systems they call.

Read each row from left to right: the method, the path with its parameters, and what the endpoint changes (*may: CommandSource…*), reads, or calls out to. Search finds an endpoint by words, a URL, a method or a table it touches. The counts in the toolbar say how many endpoints were traced all the way down and how many are covered by an API contract.

### CI/CD {#cicd}

<Shot name="cicd" title="CI/CD" source="LibreChat">
Twenty GitHub Actions pipelines, each drawn the way it runs.
</Shot>

Every pipeline in the workspace: GitHub Actions, GitLab, Jenkins, Azure, Bitbucket, CircleCI and Rancher. The list on the left says what starts each one (a pull request, a push, a tag, a schedule, a manual run). Pick one to see its jobs in order, the steps inside each, and what it produces: images, packages, deployments.

**Images** lists every Dockerfile's base image by family and version, and which pipeline builds each image. **Issues** lists what reads as a risk in the pipelines and Dockerfiles, such as an image with no version pinned.

## Structure

### Units {#units}

<Shot name="units" title="Units: how the layers lean" source="Apache Fineract">
Read as a Spring codebase: controllers over services over repositories over entities.
</Shot>

Units reads the code the way its framework does. For a Spring codebase that means controllers, services, repositories and entities; for React, components, hooks and data clients; Archstats knows the layers of most common frameworks.

- **How the layers lean**, on the left, stacks the layers so most references run down. Grey arcs run down, as they should. Red arcs are references that run back up, and the number on each says how many.
- **Where each lane lives**, on the right, is the folder tree as a map, each module coloured by its layer. Switch it to **Reach** to see code nothing reaches, or **Duplicates** for names declared more than once.
- **What it says**, below, lists the findings: layers that depend on each other both ways, modules that do too much, and the classes most used.

Click a layer or a tile to open the modules in it.

### Libraries {#libraries}

<Shot name="libraries" title="Libraries" source="Apache Fineract">
66 libraries, by how much of the code leans on each.
</Shot>

Every import from outside the project, rolled up to the depth you pick, with how many imports, files and components use it. A library used everywhere is one you are tied to; one used in two files is cheap to replace. *Looks internal* marks a name that may be the project's own code rather than a library.

### Rules {#rules}

The module rules the project declares, checked import by import, and the order of the open [lens](#lenses) if it declares one. Archstats also carries rules of its own for some ecosystems, such as "a .NET core must not depend on a plugin" or "a Go `internal/` package is not imported from outside its module"; each is marked kept, broken or not about this codebase. To fail a build when a rule breaks, use [`archstats assert`](cli.md#rules).

## How the parts depend on each other

### Connections {#connections}

<Shot name="connections" title="Connections, rolled up by folder" source="Apache Fineract">
1,156 components draw as a cloud; rolled up by folder, the shape of the package tree shows.
</Shot>

What depends on what, as a graph, a matrix, a chord diagram or a list. **Static** draws the imports; **Git** draws the parts that change together in the same commits; **Combined** shows both. Pick two parts to see every path between them.

A large codebase draws as a cloud of dots. Roll it up **by folder**, as in the picture, or by a [lens](#lenses) of your own, and the shape comes out. Click a node to inspect it; double-click to open it; shift-click to select several and make a group of them.

### Cycles {#cycles}

<Shot name="cycles" title="Cycles" source="Apache Fineract">
510 components knotted into one tangle, and a guide to the cuts that undo it.
</Shot>

A cycle is a set of parts that all reach each other through their imports, so none of them can be changed, tested or released on its own. The bar at the top lists the tangles by size; Fineract has 18, the largest holding 510 of its 1,156 components.

The matrix draws a tangle in levels, rows importing columns. Marks above the diagonal follow the levels; the ones below it run against them, and those are what make the cycle. The guide on the right takes you through the imports to cut, the one that frees the most first, and shows the files that carry each.

## Numbers and hotspots

### Metrics {#metrics}

<Shot name="metrics-summary" title="Metrics: what stands out" source="Apache Fineract">
The median component, and the patterns worth a look.
</Shot>

Every number Archstats measures, for every component, file or folder. The summary opens with the median component, then lists what stands out: where change concentrates, which metrics move together, the parts far from the rest. Each has a button to the table or plot that shows it.

<Shot name="metrics" title="The main sequence" source="Apache Fineract">
Instability against abstractness, coloured by hotspot score.
</Shot>

**Plot** draws any two metrics against each other. The main sequence preset, above, plots how much a component depends on others against how abstract it is. Components in the shaded lower-left corner are stable and concrete: much depended on, and hard to change. [Metrics](metrics.md) says what every number means.

### Hotspots {#hotspots}

<Shot name="hotspots" title="Hotspots" source="Apache Fineract">
Every component as a circle, sized by lines and coloured by hotspot score.
</Shot>

A map of the code where size is lines and colour is risk. The default, **Hotspots**, colours by the hotspot score: complex code that changes often, the code that costs the most to keep. The perspectives on the right colour the same map by other questions: churn against health, instability, code age, complex code, nesting depth, or two metrics you pick.

The numbered circles are the hottest, listed on the right. Click a ring to zoom into it.

## One part up close

### A component {#component}

<Shot name="component" title="A component" source="Apache Fineract · portfolio.loanaccount.service">
Its health, its shape, and where it sits.
</Shot>

Double-click any component, in any view, to open its page. **Reading** gives its health and hotspot score, what imports it and what it imports, where it sits on the main sequence, and the tangle it belongs to. The other tabs show its connections, its cycles, the files and units inside it, and its history.

### A file {#file}

<Shot name="file" title="A file" source="Apache Fineract · LoanWritePlatformServiceJpaRepositoryImpl.java">
Why the file scores 1.0 out of 10, line by line.
</Shot>

A file's page explains its score: each rule that took points off, the input it read and the threshold it passed. Below that, its functions by cognitive complexity. **Source** shows the code with every usage, call and reference a click away; the other tabs list its functions, its imports and its history.

## History

### Authors {#authors}

<Shot name="authors" title="Authors: who still knows the code" source="Apache Fineract · pseudonymised">
6% of the code has nobody active left who wrote it or has changed it lately.
</Shot>

**Knowledge** puts each component on one ladder: written by people still active, changed by them, changed once, or nobody active left. The map shows it across the code; the list beside it names who to ask about each part. **People** lists every contributor.

"Active" means a commit in the last 180 days, year or two years; you pick. **Pseudonymised**, as in the picture, shows *Author 148* instead of a name, in the app and in everything you export from it.

### Activity {#activity}

<Shot name="activity" title="Activity: where the work went" source="Apache Fineract">
Each component's share of the changed lines, now against the two years before.
</Shot>

**Work now** compares where the changed lines went in the last 30, 90 or 180 days, or year, with the two years before. A dot that moved far to the right is where the team's attention went. **Breadth** shows whether changes are getting wider (more files per commit), **Effort** what kind of code they land on, and **Commits** lists every commit.

### Changes and Over time {#changes}

**Changes** compares the open snapshot with an earlier one, area by area: the dependencies that are new or gone, the tangles that grew or shrank, the rules newly broken. Pick a baseline by date, tag or commit; if the baseline was read by an older analysis, Archstats offers to scan that commit again so both sides are read the same way. **Over time** puts every snapshot of the workspace on one axis, with the same readings for each.

## Your own slices and reports

### Groups and lenses {#lenses}

A **group** is a set of components or files you name yourself: a layer, a team's area, a candidate service. Select things in any view and choose **Create group**. A **lens** is a set of groups that cuts the whole codebase one way. Archstats can propose one from the names, the imports, the history, CODEOWNERS or the build modules; you correct it. Pick a lens in the sidebar and every view draws your groups instead of the folders.

### Evidence and reports {#reports}

Pin any figure or table as evidence as you go. A **report** is a notebook of your own text and evidence cells, each keeping the snapshot it came from. Start one from a template written for the codebase's ecosystem (Spring, Django, React, Go, .NET and others), print it to PDF, or save it as Markdown with its figures.

## Getting around

### SQL console {#sql}

<Shot name="query" title="SQL console" source="Apache Fineract">
Every table in the snapshot, saved queries, and the queries your reports use.
</Shot>

Read-only SQL against the open snapshot. The tables are listed on the left with their row counts; **Ctrl+Space** suggests tables, columns and metrics as you type. A result shows as a table, a chart or a plot, and can be compared with another snapshot. Save a query to keep it, or add it to a report. [Snapshot tables](snapshot.md) describes every table.

### Go to Anything {#go-to-anything}

<Shot name="goto" title="Go to Anything" source="Apache Fineract">
Files, functions and units that match "loan write", as you type.
</Shot>

Press **⌘P** (Ctrl+P), or **⇧** twice, to jump to any view, component, file, function or report by name. Start with `>` for actions, `@` for symbols, `#` for components, or `:42` for a line. **Find in code** at the bottom searches the source itself.
