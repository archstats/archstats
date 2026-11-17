---
layout: home
title: Archstats
---

# Archstats

See how a codebase is really built: its layers, its tangles, the code that costs the most to keep, the endpoints it serves and the pipelines that ship it. One scan, on your own machine, into one SQLite file you own.

<HomeActions />

<Shot class="hero" eager name="overview" title="Overview" source="Apache Fineract · 1.4 million lines of Java">
What the desktop app opens on after a scan: the size of the codebase, how healthy it is, where it tangles, and fourteen years of history.
</Shot>

## What it shows you

Every picture on this page is the desktop app on an open-source codebase. Click one to see it full size.

### The layers, and what breaks them

<Shot name="units" title="Units" source="Apache Fineract">
Controllers over services over repositories over entities. The red arcs are the 66, 5 and 13 references that run back up the stack; the map shows where each layer lives in the folders.
</Shot>

Archstats reads the code the way its framework does, so a Spring codebase comes back as controllers, services, repositories and entities, and a React one as components, hooks and data clients.

### The tangles, and how to undo them

<Shot name="cycles" title="Cycles" source="Apache Fineract">
510 of 1,156 components reach each other through their imports. The guide on the right lists the imports to cut, the one that frees the most first.
</Shot>

### The code that costs the most to keep

<Shot name="hotspots" title="Hotspots" source="Apache Fineract">
Each circle is a component, sized by its lines and coloured by how complex it is and how often it changes.
</Shot>

### What the system serves, and how it ships

<Shot name="http" title="HTTP" source="Apache Fineract">
817 endpoints, each followed down through controllers, services and repositories to the tables it reads and writes.
</Shot>

<Shot name="cicd" title="CI/CD" source="LibreChat">
Twenty GitHub Actions pipelines, each drawn the way it runs: what starts it, its jobs and steps, and what it builds.
</Shot>

### Who still knows the code

<Shot name="authors" title="Authors" source="Apache Fineract · pseudonymised">
For each part of the code: is anyone who wrote it, or changed it lately, still active? Names can be swapped for *Author 148* everywhere.
</Shot>

That is six of sixteen views. The [Tour of the app](tour.md) shows them all, with how to read each.

<StatStrip />

## Three ways to read a snapshot

Everything Archstats finds goes into one SQLite file, the **snapshot**. You read it the way that suits you:

<WaysIn />

## What it reads

| Languages | Read with |
|---|---|
| Java, Kotlin, C#, Go, Python, PHP, JavaScript, TypeScript (with Vue and Svelte), Swift, Objective-C, Dart | tree-sitter: units, imports resolved to components and modules, cognitive complexity per function |
| Scala | regular expressions: packages and imports |

Modules come from the project's own manifests: go.mod, Maven, Gradle, npm workspaces, Composer, .csproj, SwiftPM, Xcode and pubspec. CI/CD comes from GitHub Actions, GitLab, Jenkins, Azure, Bitbucket, CircleCI and Rancher pipelines, and from Dockerfiles. [Ecosystems](ecosystems.md) explains how each one is read.

Archstats runs on your machine. Nothing is uploaded, there is no account, and the AI features stay off until you turn them on.
