# 0017. Module Boundaries from Manifests

- **Status**: Accepted
- **Date**: 2026-09-21

## Context and Problem Statement

ADR 0007 made the component a first-class concept, resolved either from a declaration in the source (a Java package, a C# namespace) or from the directory tree. That answers "where does this code say it lives". It does not answer "what does this project build and publish", and for several ecosystems the second question is the one an architect is actually asking.

Measured on real projects:

- nopCommerce is 3,650 C# files whose namespaces say `Nop.Core`, `Nop.Services`, `Nop.Plugin.Payments.PayPal`. What makes it a plugin architecture is its 40 `.csproj` files, and nothing in any namespace says which module is a plugin.
- Sylius is 60 composer packages whose entire design rests on `Sylius\Component\*` never depending on `Sylius\Bundle\*`. The split is declared in `composer.json` and in where each package sits, not in any namespace.
- LibreChat declares `api`, `client` and `packages/*` as npm workspaces. No source file mentions them.
- Exposed is 50 gradle modules; django-oscar is 29 Django apps; Broadleaf is 13 maven modules.

`extensions/components/declbased/aliases.go` already reads `package.json` and `tsconfig.json`, because guessing a monorepo's own import names from directory suffixes fails on every monorepo. That was the right idea scoped to one ecosystem.

## Decision Drivers

- **Evidence over guessing**: a module boundary that a project writes down should be read, not inferred from names.
- **Existing numbers must not move**: components are what every current metric is computed from, and a silent change to them would invalidate every saved snapshot and every user's mental model.
- **One walk**: the walker already honours `.gitignore` and `.archstatsignore` (ADR 0012), and a second, independent walk would not.

## Considered Options

- **Extend component resolution**: make manifests a third resolution strategy and fold modules into components.
- **Modules as a separate first-class concept**: read manifests into their own map, indexed alongside files and components, leaving component resolution untouched.
- **Per-ecosystem extensions**: let each language extension read its own manifests.

## Decision Outcome

Chosen option: **modules as a separate first-class concept**, in `core/module`, with one `Reader` per ecosystem (dotnet, composer, node, gradle, maven, go, django) and a `Map` that answers which module owns a file.

Modules are read in `core`, not in an extension, because components, rules and views all need the same answer and none of them should learn an ecosystem's name — the same reasoning as ADR 0007's pluggable resolvers. `Results` gains `Modules`, `FileToModule` and `ModuleToFiles` beside `FileToComponent` and `ComponentToFiles`.

A module is **not** a component. Component resolution is unchanged, so every existing metric produces exactly the number it did before.

`ReadFrom` takes the file list the walker produced rather than walking for itself. Walking raw, archstats reported *itself* as 84 modules, because it ignores `**/temp_testdata/**` and that directory holds full checkouts of MediatR, kotlinx-datetime and elepy for the e2e suite. It has three `go.mod` files.

### Consequences

- **Good**: the real boundary of a .NET, Symfony, npm-workspace, gradle or Django project is available for the first time, and with it the rules in ADR 0019.
- **Good**: no existing number changes.
- **Good**: a project that declares nothing yields an empty map, which is the common and correct answer for a single-package repository.
- **Bad**: two concepts that sound alike now coexist, and "component" versus "module" has to be explained in the UI.
- **Bad**: manifests are parsed on every run. Cheap in practice, but it is work that did not happen before.
