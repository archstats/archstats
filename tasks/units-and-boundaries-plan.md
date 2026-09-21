# Units and Boundaries

Bringing every supported ecosystem to the depth Java has, in two releases.

Java's depth today rests on a coincidence: one file is one public type, and
the path encodes the package and the name. That makes the file a legitimate
stand-in for the type, so `java_full_class` can be a per-file stat with
`LastRecordStatMerger` and every view downstream can key on file. No other
supported ecosystem honours that coincidence.

Measured, on real projects, before any of this was written:

| Project | Ecosystem | Types | Functions outside types |
|---|---|---|---|
| BroadleafCommerce | Java | 2,871 | — |
| nopCommerce | C# | ~3,650 files, **1,567 partial classes** | — |
| Sylius | PHP | 3,997 | — |
| django-oscar | Python | 837 | 191 |
| Exposed | Kotlin | 1,319 | **696** (471 top-level, 225 extension) |
| gin | Go | 178 | **1,344** |
| archstats | Go | 106 | **556** |
| LibreChat | TS/React | **14** | **997** |

A Classes view on LibreChat shows fourteen things out of a thousand.

## Principles

Taken from how files and components were built, and binding on everything below.

1. **Pluggable providers, neutral consumers.** ADR 0007 is the precedent:
   components resolve by declaration *or* by directory, and the metric layer
   does not know which produced them. Units and boundaries follow the same
   shape — ecosystems contribute evidence, readings stay ignorant of language.
2. **Pre-aggregated indexes.** Every relationship lands in `core.Results` as a
   map built once, beside `FileToComponent` and `ComponentToFiles`.
3. **Anything user-visible has a definition yaml**, embedded with `go:embed`,
   flowing into the `definitions` view and `docs/metrics.md` via `cmd/gendocs`.
4. **Three test layers**, as the tree-sitter packs already have them.
5. **Comments name the codebase that exposed the need.** The house style, and
   the reason the C# file-scoped namespace and express `require()` fixes are
   readable a year later.
6. **Assert the silent failure.** The bug class here never errors — it returns
   a confident wrong graph. Every e2e assertion names the tell.

---

# Release A — Boundaries and Edges

No new grain. Uses files, components, snippets and connections as they are.
Delivers each ecosystem's real module boundary and its architectural rule.

## A1. Module map from manifests

`extensions/components/declbased/aliases.go` already reads `package.json` and
`tsconfig.json` to learn what a project calls its own packages. It is the right
idea scoped to one ecosystem. Promote it to `core/module`: a manifest reader
registry, one reader per ecosystem, producing a `ModuleMap` of
`path prefix -> module name` plus the module's declared dependencies.

Readers and what they unlock:

| Reader | Manifest | Measured |
|---|---|---|
| dotnet | `*.csproj`, `*.sln` | nopCommerce: 40 projects — `Nop.Core`, `Nop.Services`, `Nop.Plugin.*`. **The plugin architecture is the architecture** |
| composer | `composer.json` PSR-4 | Sylius: 60 packages; the `Bundle` (24) vs `Component` split |
| gradle | `build.gradle.kts`, `settings.gradle.kts` | Exposed: 50 modules |
| node | `package.json` workspaces | LibreChat: `api`, `client`, `packages/*` |
| django | `INSTALLED_APPS`, `apps.py` | django-oscar: 29 apps |
| go | `go.mod` + `internal/` | module path and enforced boundaries |
| maven | `pom.xml` | Broadleaf: 13 modules |

Exposed as a third component resolution strategy beside `declared` and
`directory`: `manifest`, and in `fallback` order manifest → declared →
directory. This is ADR 0007's pluggable-resolver decision extended, not
replaced.

## A2. Typed edges

`component.Connection` gains a `Kind`. Kinds, each with a definition yaml:

- `import` — an ordinary static import. Today's only kind.
- `type_only` — erased at compile time, **not** runtime coupling.
  LibreChat: 681 of 5,100 imports (13%). Counting them inflates every
  TypeScript graph by an eighth.
- `dynamic` — resolved from a string at runtime.
- `embed` — Go struct embedding, Kotlin delegation. gin embeds `RouterGroup`
  and `ResponseWriter`; no `implements` query can see it.
- `manifest` — module A declares a dependency on module B without any import.

Existing views keep their current totals by defaulting to runtime kinds, so no
number an existing user reads changes silently. `type_only` is excluded from
coupling by default and available as a filter.

## A3. Unresolved edges, reported

The finding that most changes what archstats owes its user:

> django-oscar makes **746** `get_class()` / `get_model()` calls against
> **1,645** static imports. Roughly 31% of its dependency edges are resolved
> from strings at runtime — and that is not sloppiness, it *is* Oscar's
> extensibility architecture. Today archstats draws Oscar's graph missing a
> third of it and says nothing.

Add an `unresolved_edges` view and a `modularity__edges__unresolved` stat:
file, line, the expression, and why it could not be resolved. Ecosystem
detectors: Python `get_class`/`get_model`/`import_module`/`__import__`,
JS/TS dynamic `import()` with a non-literal, PHP string class-name `new`,
Symfony autowiring by type-hint, .NET reflection loading.

Where the pattern is mechanical — Oscar's `get_class("catalogue.views",
"ProductDetailView")` is a two-argument lookup — resolve it into a real edge
with kind `dynamic`. Where it is not, count it and say so. An honest "31% of
this codebase's edges are dynamic and not drawn" is worth more than a
confident wrong graph.

## A4. Rules

One or two per ecosystem, each the rule that decides whether the architecture
is intact. A rule is a named, checkable statement over the module map and
typed edges, reported as violations with evidence.

| Ecosystem | Rule |
|---|---|
| PHP / Sylius | `Component` must never depend on `Bundle` — the framework-agnostic domain must not know the framework glue |
| C# / nopCommerce | Core must never depend on a plugin; plugins may depend on core |
| Python / Django | App isolation — which of the 29 apps reach into each other, **including** through `get_class` |
| Go | Who imports whose `internal/` — compiler-enforced, so stronger evidence than any ordinary edge |
| Kotlin | Gradle module boundaries and `@InternalApi` (26 uses in Exposed) — what is public surface |
| TypeScript | Workspace boundaries, with runtime and type-only edges kept apart |
| Java / Spring | Layering — already built, moves from hardcoded UI logic into a declared rule |

Rules live in `extensions/rules/`, one yaml per rule with the same definition
shape as metrics, so they document themselves and reach `docs/`. A `rules`
view lists every violation.

## A5. Docs for Release A

- **ADR 0017** Module boundaries from manifests — the third resolver.
- **ADR 0018** Typed dependency edges — why `type_only` is excluded by default.
- **ADR 0019** Reporting unresolved edges — the honesty decision, with the
  Oscar numbers as the driver.
- Definition yaml for every new stat, kind and rule.
- `docs/ecosystems/<name>.md`, one per ecosystem: what its unit is, where its
  role is declared, where its module boundary lives, which edges are invisible
  and why, and its rules. Written from the measurements above.
- `ARCHITECTURE.md` gains the module map and edge kinds.

---

# Release B — Units

The new grain. Go and TypeScript get a meaningful unit view; C# gets a correct
one.

## B1. The Unit grain in core

A third grain between component and snippet: a named, addressable thing.
Snippets stay what they are — observations with no identity. Units are
entities.

```go
// core/unit/unit.go
type Unit struct {
    ID        string   // "Nop.Core.Domain.Customer", "github.com/gin-gonic/gin#Context.JSON"
    Kind      string   // type | function | module | route
    Name      string
    Files     []string // plural: nopCommerce has 1,567 partial classes
    Component string
    Module    string   // from the Release A module map
    Owner     string   // receiver, enclosing type, extension receiver
    Markers   []Marker
}

type Marker struct {
    Source string // annotation | attribute | struct_tag | directive | filename | path | manifest
    Key    string
    Value  string
}
```

Three properties, each forced by a measurement:

- **Kind varies.** gin is 1,344 functions to 178 types; LibreChat 997 to 14.
- **Files is many-to-many.** 1,567 partial classes, and 141 nopCommerce files
  hold several top-level types. A per-file stat cannot represent either.
- **Owner is separate from location.** Exposed has 225 extension functions;
  `fun Table.selectAll()` belongs to `Table` and lives elsewhere. gin has 430
  methods whose receiver is discarded today.

`Marker` is deliberately untyped because the role is not declared in the same
place twice:

| Ecosystem | Where the role lives | Evidence |
|---|---|---|
| Java | annotation on the type | `@Component` 368, `@Service` 280, `@Entity` 157 |
| C# | the `.csproj` | attributes are display/serialization noise — `NopResourceDisplayName` 2,609 |
| PHP | interfaces + composer package | 344 interfaces : 663 classes in `Component`; top attributes are Behat steps |
| Python | **the filename** | `apps.py` 29, `views.py` 25, `models.py` 24, `forms.py` 20; only 39 architectural decorators in the repo |
| Kotlin | gradle module + naming | top annotations are `@Suppress`, `@JvmName`, `@OptIn` |
| Go | package + function signature | zero annotations; 316 `HandlerFunc`, 150 `form:` tags |
| TS/React | filename and path | decorators exist only in Angular and Nest |

For Django the filename *is* the annotation. Path and manifest membership are
marker sources on equal footing with annotations.

### Core plumbing

- `Analyzer.RegisterUnitProvider(UnitProvider)` beside the existing registrars.
- `file.Results` carries `Units`, merged like snippets in `mergeFileResults`.
- `core.Results` gains the pre-aggregated indexes, in the house style:
  `Units`, `UnitsByFile`, `UnitsByComponent`, `UnitsByKind`, `UnitToComponent`,
  `FileToUnits`, `UnitGraph`.
- Partial-class merge happens in aggregation: units with equal `ID` fold into
  one, unioning `Files` and `Markers`.

## B2. Unit providers

One per language pack, emitting units alongside today's snippets. Nothing
existing is removed.

| Language | Units emitted | The specific difficulty |
|---|---|---|
| Java | types | reference implementation; `java_full_class` kept as an alias |
| C# | types, folded across partials | `partial` and multi-type files, the whole point |
| Kotlin | types, objects, top-level funcs, extension funcs | receiver → `Owner` |
| Go | packages, types, funcs, methods | receiver → `Owner`; embedding as `embed` edges; implicit interface satisfaction by method-set matching |
| TypeScript / JS | modules, classes, exported funcs, components | barrel files (77 in LibreChat) launder the source; `import type` |
| Python | modules, classes, module-level funcs | filename as marker; `get_class` targets as units |
| PHP | types, traits, interfaces | PSR-4 for `ID` |
| Scala | types, objects | stays regex; units only where regex reaches |

Go's implicit interface satisfaction is the one genuinely new algorithm:
nothing declares it, so it is computed by matching method sets across units in
the module, and emitted as `implements` edges with kind `embed`.

## B3. Views

- `units` — the new table. Columns: id, kind, name, component, module, owner,
  files, marker summary.
- `unit_connections` — unit→unit with edge kind, superseding
  `java_class_connections_direct`.
- `java_class_connections_direct` / `_indirect` become aliases that select
  `kind = 'type'`, so nothing breaks.
- The indirect view's `DijkstraAllPaths` over every pair is already the
  slowest thing in the repo. Before it meets a 3,650-unit C# codebase it gets a
  hop cap and a unit-count cutoff, both configurable, both documented.

## B4. UI

- `loadRawClasses` → `loadUnits`, reading `units` rather than `files`.
- `pages/views/java/classes.vue` → `pages/views/units.vue`, redirect preserved.
- Nav group "Java" → "Units"; `isJavaProject` → `hasUnits`.
- `javaFrameworks.ts` → `frameworks/`: the engine (`detect`, `classify`,
  `LaneDef`) unchanged, plus per-ecosystem profile sets. `classify()` is
  already kind-agnostic — only `isRecord`/`isInterface` are type-specific, and
  a React component is a unit with `kind: function`, a `path` marker and
  imports.
- `STRUCTURE` stays the universal fallback, so every ecosystem has a working
  view before a single profile is written.
- Groups accept units of any kind, so create-from-selection works in the new
  view exactly as elsewhere.
- Snapshots without a `units` table fall back to the `files` +
  `java_full_class` path — what the `legacy` field in the profiles was for.

## B5. Docs for Release B

- **ADR 0020** Unit as a First-Class Concept — the sibling to 0007, same
  shape, with the measured table as its decision drivers.
- **ADR 0021** Markers as untyped evidence — why filename and path rank with
  annotations.
- Definition yaml per unit kind, marker source and new stat.
- `docs/ecosystems/<name>.md` gains its unit section.
- `ARCHITECTURE.md` gains the third grain and the aggregation rules.

---

# Tests

Three layers, as the packs already have them. Every assertion names the silent
failure it guards.

## Layer 1 — provider unit tests

Per language, against embedded fixtures, in the style of
`extensions/treesitter/csharp/csharp_test.go`: exact expected sets, not counts.

New fixtures, each written in the dialect that currently breaks:

- `TestPartial.cs` — one type across three files, plus a file with four
  top-level types. Asserts one unit, three files.
- `TestExtensions.kt` — extension functions and top-level functions. Asserts
  `Owner` is the receiver, not the file's own type.
- `TestHandlers.go` — methods with receivers, struct embedding, a type that
  satisfies an interface without saying so. Asserts the implicit `implements`.
- `TestComponents.tsx` — function components, a barrel re-export, and
  `import type`. Asserts the type-only edge is kind `type_only` and the
  barrel resolves to the true source.
- `test_app/` — Django `models.py` / `views.py` / `apps.py` with a
  `get_class` call. Asserts filename markers and the resolved dynamic edge.

## Layer 2 — algorithm tests

`.txt` fixtures beside the code, as `core/component/shortest_cycles_*_test.txt`
already do:

- partial-class folding
- module map resolution per manifest reader
- Go method-set matching for implicit interfaces
- barrel-file resolution
- rule evaluation, one fixture per rule, violations and clean

## Layer 3 — e2e against real projects

`e2eTest/languages_e2e_test.go` extends to the projects measured here, run
through the real CLI with default flags. Each fixture asserts the specific
silent failure:

| Fixture | Asserts |
|---|---|
| nopCommerce | 40 modules from `.csproj`; the partial classes fold to fewer units than files; core→plugin rule reports zero violations |
| Sylius | 60 composer packages; `Component` does not depend on `Bundle` |
| django-oscar | 29 apps; ~746 dynamic lookups found, most resolved; unresolved count reported and non-silent |
| LibreChat | workspace boundaries; ≥900 function units; type-only edges excluded from coupling and the runtime total lower than the naive total |
| gin | ≥1,300 function units; `RouterGroup` embedding as an `embed` edge; at least one implicit interface satisfaction found |
| Exposed | 50 gradle modules; extension functions carry a receiver `Owner` |
| Broadleaf | unchanged from today — the regression guard that Java stays exact |

Plus the existing tell, kept and extended: no files in `Unknown`, and now also
no ecosystem resolving to zero units where units exist.

---

# Sequencing

**A1 → A2 → A3 → A4**, then **B1 → B2 → B3 → B4**. Release A ships and is
useful on its own; nothing in it is rework for B, because the module map and
edge kinds are what units hang off.

Within B2, order by evidence of current harm: **TypeScript** (14 units visible
of ~1,000, 13% edge inflation, and you dogfood it), then **Go** (same, and it
is this repo), then **C#** (`.csproj` alone unlocks the plugin reading), then
Python, Kotlin, PHP, Scala.

Java moves last and only to aliases, so the reference implementation stays the
regression guard throughout.

# Risks

- **The indirect connections view.** `DijkstraAllPaths` over every unit pair,
  on a 3,650-unit codebase. Capped and cut off before B3 ships, not after.
- **Snapshot compatibility.** Old `.db` files have no `units` table. The UI
  fallback path is written in B4 and tested against a snapshot produced by the
  current build.
- **Go implicit interfaces.** The one new algorithm, and the one with no
  ground truth in the source to check against. Fixture-driven, and deliberately
  conservative: report satisfaction only within the module.
- **Marker noise.** Sylius has 8,028 attributes, 5,898 of which are Behat test
  steps. Markers need per-ecosystem allow/deny lists or the lanes drown.

---

# Status — 2026-09-21

**Release A: done and verified.** **Release B: B1 landed, B2–B5 not started.**

| Item | State | Verified against |
|---|---|---|
| A1 module map | done | nopCommerce 40 `.csproj`, Sylius 60 composer, LibreChat 7 workspaces, Exposed 50 gradle, Broadleaf 13 maven, django-oscar 39 apps |
| A2 typed edges | done | LibreChat: 293 of 2,862 component edges are `type_only` and out of coupling |
| A3 unresolved edges | done | django-oscar: 664 dynamic references now resolved, 17 reported. ~41% of its graph was invisible |
| A4 rules | done | 3 rules shipped; 0 violations in nopCommerce, 1 real violation in Sylius |
| A5 docs | done | ADRs 0017–0019, `docs/ecosystems.md`, `docs/metrics.md` regenerated (72 definitions, git metrics no longer missing) |
| B1 unit grain | done | ADR 0020; nopCommerce 3,862 units from 3,650 files, 18 spanning several |
| B2 providers | Java + C# only | Go, TypeScript, Kotlin, Python, PHP, Scala outstanding |
| Bug fixes | done | alias leak, enums as types, linker/view component agreement |
| B3 views | `units` only | `unit_connections` outstanding |
| B4 UI | not started | |
| B5 docs | ADR 0020 only | ADR 0021 (markers as evidence) outstanding |

Full suite green apart from `Test_E2E_DirectoryStrategy_RealFiles_Advanced`,
which was **already failing before this work** on uncommitted changes to
`core/component/shortest_cycles.go`, `extensions/components/graph_metrics.go`
and `shortest_component_cycles_view.go`. Verified by stashing only the files
this work touched and re-running.

## Bugs found and fixed

- **Aliases leaked from ignored directories.** `readAliases` walked the tree
  itself with its own skip list rather than the walker's ignore handling
  (ADR 0012). Run against archstats' own repository it found 23 alias
  entries, among them `elepy-vue` pointing into `e2eTest/temp_testdata/`, a
  vendored checkout the repository explicitly ignores -- so an import could
  resolve into a directory the user excluded on purpose. Now reads the file
  list, exactly as `core/module` does. Regression tests in
  `extensions/components/declbased/aliases_test.go`.

- **An enum was not a type.** The `modularity__types__total` queries for Java
  and C# covered class, interface, record and struct but not enum. This
  undercounted nopCommerce by 143 types of 4,067 and elepy by 13 of 372, and
  because abstractness is abstract types over total types, it **overstated
  abstractness** everywhere enums are common. Java enums are now units too.
  **This moves an existing metric**: abstractness falls slightly for Java and
  C# projects, so snapshots taken before and after are not comparable.

- **The linker and the view disagreed about what a component is.** Dynamic
  lookup resolution accepted any directory the walker had seen, while
  `unresolved_edges` required a directory that had actually produced
  snippets. django-oscar's template trees hold thousands of files and no
  snippets, so a lookup naming one resolved here and was then reported as
  unresolvable there -- printing the resolver's intermediate form
  (`nowhere/at/all`) rather than what the code wrote. Both now read the
  components that will actually exist.

## Investigated and found not to be a bug

- **Root-level source files are analysed.** I reported earlier that the
  `**/*.ts` style globs skip files at the repository root. They do not: the
  walker emits those as `./index.ts`, with a separator, which the patterns
  match. The earlier probe used bare filenames, which is not the path format
  the walker produces. Pinned by `e2eTest/rootlevel_e2e_test.go`, because the
  behaviour depends on two things agreeing that nothing states out loud.

## Test coverage added

| Layer | Where |
|---|---|
| Unit grain | `core/unit/unit_test.go` -- folding, ordering, evidence survival, no input mutation |
| Module readers | `core/module/module_test.go` -- seven ecosystems, malformed and anonymous manifests, directory boundaries, order independence |
| Edge kinds | `core/component/connections_test.go` -- kinds carried, erased imports excluded from the graph, phantom targets and self-references rejected |
| Rules | `extensions/rules/rules_test.go`, `check_test.go` -- every shipped rule has a case that must fire and one that must not; manifest-only edges; ecosystem leakage |
| Views | `unresolved_edges_view_test.go`, `module_view_test.go` -- shape, empty cases, own packages versus third-party |
| Aliases | `declbased/aliases_test.go` -- the leak, and parity between the two readers |
| C# units | `csharp_test.go` -- partial classes fold, several types per file stay apart, attributes attach to the right one |
| Architecture e2e | `e2eTest/architecture_e2e_test.go` -- each rule fires through the real CLI on a fixture built to break it |
| Root-level e2e | `e2eTest/rootlevel_e2e_test.go` |
| Real projects (default) | `e2eTest/real_projects_e2e_test.go` -- Polly, celery, zustand |
| Real projects (heavy) | `e2eTest/real_projects_heavy_e2e_test.go` -- Sylius, django-oscar, behind `-tags heavy_e2e` |

## Fixture cost

Default `go test ./...` fetches **59MB** of checkouts; `go test -tags heavy_e2e`
adds 186MB more. The first cut of this suite used nopCommerce, Sylius and
django-oscar and cost 320MB on every run.

What the search found, measured rather than guessed:

| Slot | Was | Now | Why |
|---|---|---|---|
| C# | nopCommerce 131MB | **Polly 14MB** | Strictly better: 38 units span several files against nopCommerce's 18, one across eight. Polly has no plugin projects, so the .NET plugin rule is covered by `architecture/dotnet` instead |
| Python | django-oscar 80MB | **celery 11MB** | Weaker and worth stating: 6 dynamic references against Oscar's 664. Proves the mechanism reaches a real codebase; the scale evidence stays in the heavy tier |
| PHP | Sylius 107MB | Sylius, heavy tier | Nothing smaller works. SyliusGridBundle is 2.3MB with the same layout on disk, but its composer.json sits at the root, so the module's directory is "" and the rule cannot see the split |

Rejected after measuring: `django-allauth` 13MB resolved **zero** dynamic
edges; `scrapy` 8.7MB resolved 2; `wagtail` has Oscar's volume at 68MB, which
saves nothing; `quartznet` 45MB and `SimplCommerce` 408MB are larger than
Polly with no more evidence; `SyliusMailerBundle` and `SyliusResourceBundle`
have the Component/Bundle layout but the rule cannot read it.

## Tried and reverted

**Matching rules on path segments rather than module directories.** Intended
to let the Symfony rule read Sylius's split-repo layout, where the module
directory is "". It reported **sixty violations in Sylius instead of one**:
`(?i)/component/` also matches `Bundle/AdminBundle/Twig/Component/`, a Twig
UI folder with nothing to do with a domain package. Reverted, and the reason
is recorded in `rule.go` so it is not tried again. The split-repo layout
stays unread, which is the honest outcome: a rule reads what a project
declares, not what a directory happens to be called.
