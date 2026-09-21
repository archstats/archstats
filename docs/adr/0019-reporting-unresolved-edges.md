# 0019. Reporting Unresolved Edges

- **Status**: Accepted
- **Date**: 2026-09-21

## Context and Problem Statement

django-oscar makes **746** `get_class()` and `get_model()` calls against **1,645** static imports. Roughly a third of its dependency graph is resolved from strings when the program runs, and that is not sloppiness — it is Oscar's extensibility model. Every app can be forked and overridden, so nothing may import another app's classes directly.

Read as static imports only, archstats drew Oscar's graph missing a third of it, reported success, and said nothing. This is the same failure class the real-project suite was built for: the analyser runs, the numbers look plausible, and they describe a codebase that does not exist.

The same shape appears elsewhere: dynamic `import()` with a computed specifier, PHP class names built from configuration, .NET reflection loading, Symfony autowiring by type-hint.

## Decision Drivers

- **A confident wrong graph is worse than an admitted gap.** An architect can weigh "31% of these edges are dynamic"; they cannot weigh a number that is quietly short.
- **Resolve what is mechanical.** `get_class("catalogue.views", "ProductDetailView")` is a two-argument lookup naming a module. Refusing to resolve it on principle would be its own kind of dishonesty.
- **Never invent an edge.** A lookup naming something this analysis never saw must not be guessed into a component.

## Considered Options

- **Ignore dynamic lookups**: the previous behaviour.
- **Resolve everything possible and stay silent about the rest**: better graph, same silence.
- **Resolve what is mechanical, and report the remainder as its own result.**

## Decision Outcome

Chosen option: **resolve what is mechanical, report the rest.**

Language packs capture dynamic lookups as `modularity__component__imports__dynamic`. Because the captured value is a module path, the existing component linker resolves it like any other import, and the edge enters the graph with kind `dynamic` (ADR 0018). What does not resolve appears in a new `unresolved_edges` view with the file, the line, the expression and why it could not be placed.

One narrow extra step was needed: Django's `get_model("catalogue", "Product")` names an *app label*, a single bare segment, which every resolution branch ignores because they all want a dot or a slash. Resolving a bare segment against directory names — only for dynamic lookups — is what took Oscar from 169 resolved edges and 469 unresolved to **342 resolved and 17 unresolved**.

Measured on django-oscar: 973 static references and 667 dynamic ones. Roughly **41% of that codebase's dependency graph was invisible** and is now drawn, with 17 lookups reported rather than dropped.

### Consequences

- **Good**: the first honest graph of a codebase built on runtime lookup.
- **Good**: the remaining gap is a number on screen instead of an absence.
- **Bad**: dynamic edges are inferred from a string, so they are weaker evidence than an import, and the kind is the only thing saying so.
- **Bad**: the bare-segment resolution is a directory-name match. It is confined to dynamic lookups precisely so an ordinary `import os` can never be captured by a folder called `os`, but it remains the loosest resolution in the codebase.
