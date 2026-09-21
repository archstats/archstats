# 0018. Typed Dependency Edges

- **Status**: Accepted
- **Date**: 2026-09-21

## Context and Problem Statement

Every edge archstats recorded was the same kind of thing: a static import, counted once, feeding coupling. Real codebases do not have one kind of dependency.

- **TypeScript erases `import type`.** It is a real dependency on a shape and no dependency at all once the program runs. LibreChat writes 681 of its 5,100 imports this way; 293 of its 2,862 component edges. Counted as coupling, they inflate the graph by roughly a tenth.
- **Some dependencies are resolved from strings at runtime.** django-oscar makes 746 `get_class`/`get_model` calls against 1,645 static imports (ADR 0019).
- **Go composes rather than references.** gin embeds `RouterGroup` and `ResponseWriter`; no import and no `implements` query sees either.
- **Manifests declare dependencies with no import anywhere.** A `.csproj` `ProjectReference` is how a .NET plugin reaches core.

`component.Connection` already had an unused `Type` field.

## Decision Drivers

- **Coupling must mean coupling**: a metric that counts erased imports is measuring something nobody asked about.
- **Nothing should be dropped silently**: a type-only dependency is real and an architect may want to see it.
- **Existing numbers**: an unclassified edge must keep behaving exactly as it did.

## Considered Options

- **Ignore erased imports entirely**: simplest, but loses a real dependency the user may care about.
- **Keep counting them**: what the code did, and what the enterprise fixture asserted until now.
- **Give edges a kind, and decide per kind what counts.**

## Decision Outcome

Chosen option: **edges carry a kind** — `import`, `type_only`, `dynamic`, `embed`, `manifest` — in the existing `Connection.Type` field, with `KindImport` as the zero value so an edge nobody classified is an ordinary import and every existing number is unchanged.

`Results.Connections` holds the edges that exist at runtime and is what the component graph, and therefore every coupling metric, is built from. `Results.AllConnections` holds everything, and `component_connections_direct` reads it and names each edge's kind.

The enterprise fixture previously asserted that `import type` counts, "because a type-only dependency is still one". It is a dependency — but it is not coupling, and the fixture now asserts the distinction.

### Consequences

- **Good**: TypeScript coupling numbers describe what actually couples.
- **Good**: nothing is dropped; a type-only edge is visible, labelled, and excluded from the maths.
- **Bad**: `Connections` and `AllConnections` are easy to confuse, and picking the wrong one is a silent error.
- **Bad**: one shipped assertion changed meaning, so snapshots taken before and after are not comparable for TypeScript projects.
