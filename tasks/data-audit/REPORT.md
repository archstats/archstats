# Data truth audit — 2026-09-23 (two passes)

**Pass 1:** traced every number the UI shows to its source and recomputed it from the rawest table
(and, for unit edges, from the source text).
**Pass 2:** tried to break pass 1. It rescanned six codebases with the *current* engine working tree through a
scanner that mirrors the app's scan path exactly (`appscan.main.go`); re-ran the Units view's real
pipeline (the app's own `loadUnits` / `classify` / `buildModuleGraph` / `findingsFor` / `readRelationship`
/ `blastRadius`) and the Connections resolver on real data; checked git against the repositories
themselves; and toured the UI (Sonnet drove the browser; every flagged state was re-screenshotted and
checked here).

Codebases: Broadleaf (Java/Spring), elepy (Java), LibreChat (TS/JS), gin (Go), django-oscar (Python),
nopCommerce (C#). Kotlin and PHP not covered. Broadleaf's current-engine rescan is identical to its
2026-09-22 snapshot, so every finding below describes today's code.

## Verified correct
Files, lines (exact per file), components, directories; the component graph (reachability, paths,
strongly connected groups, cycle set, cycle co-changes); coupling (dependents/dependencies; afferent/
efferent as file counts; instability); code-health formulas and component roll-ups; Java file ->
component; Java/C# unit edges from explicit imports (100% precision on 600 sampled; C# same-namespace 97%).

## Wrong — major
| # | What | Evidence | Pass 2 |
|---|---|---|---|
| E9 | Java unit edges come only from import statements (`java.go` javaRefs) | same-package `extends`/`implements` from the engine's own markers: Broadleaf 928/928 missing, elepy 45/45 | confirmed, exact method, current engine |
| E10 | Go: only cross-package function calls resolve | gin: 10 edges for 1,531 units | confirmed, current engine |
| E8 | Java `@interface` types are never units | Broadleaf: 762 imports of 63 annotation types dropped | — |
| E11/E29 | TS/JS/Python: module-level usage is not attributed to any unit | LibreChat: 191 imports lost; Python: 44% of absolute imports | Python new |
| E28 | Vendored/minified JS declared as units | min.js + vendor folders: Broadleaf 19% of units, nopCommerce 46% (`$`, `$e`, `$i`) | new |
| U8 | Connections Files level draws no import edges | real resolver: 0 edges on 4 languages; engine already resolves 9,561 / 10,259 / 1,391 / 417 file dependencies in `unit_connections` | confirmed |
| U25 | Opening any component rebuilds all edges from raw snippets | C#: 4,623 of 4,652 edge weights change, references double (10,489 -> 20,909); Java/Go unaffected | new |
| U1 | "N modules are never imported" counts isolated modules | real code: Broadleaf 370 vs 1,467; LibreChat 319 vs 603; gin 86 vs 94 of 96; nopCommerce 741 vs 1,814 | confirmed on 6 |
| U16 | Lead Units finding is a fallback-lane artifact | real classifier: 978 of 1,099 Broadleaf boundary refs land on modules in "Services & Other" only by fallback; LibreChat 242 of 270 | confirmed |
| U22 | "up to 6 hops" is the search cap | Broadleaf: 270 modules reach further; worst misses 342 of 708 | confirmed |
| E25/E26 | Git history is every branch (`git log --all`), duplicates counted | Broadleaf: 3,367 commits only on other branches; 1,456 (8%) same change on several branches | new |
| E27 | Files whose every commit touched >100 files read as brand new | Broadleaf 193 files, e.g. CountryDao.java (2011, 13 bulk commits) shows age 0 | new |

## Wrong — smaller
- E1 React detection = any capitalised function: 263 in Broadleaf, 1,109 in nopCommerce (neither uses React).
- E32 Hotspot #1 is vendored `redactor.js`; #4 a jquery-ui file with one commit; CSS is scored. Scores are relative to the hottest file.
- E31 Authors identified by name: Broadleaf 141 names ≈ 120 people; Jeff Fischer's top partner is "jefffischer" (himself).
- E16/U12 Deleted files carry component `''`: 115/141 authors' Components +1 (all-deleted authors show "Components 1"); partners matched on it.
- E18/U13 Unscored files stored as health 0 (below the 1–10 floor) and shown red.
- E23 `.git/` internals scanned as source (23–24 files per repo; 24 of gin's 153).
- E5 `shortest_path_length` / furthest distance count nodes: component page shows "Furthest reach 8 hops" next to a 7-hop path.
- U18 File Imports page: "IMPORTS 35" vs Java tab "19"; "IMPORTED BY 194" vs Java tab "172" (22 import a different `Order`).
- U24 "Crowded" finding leads with moment.js / chart.js / a test file.
- U20 Group sizes estimated by file share (median error 16–21%) though exact lines exist.
- E3 `java_class` last-record merger prints one arbitrary class as a codebase metric.
- E12 age = days since FIRST commit; description says "since last modified"; roll-up is the mode.
- E7, E30 Metric descriptions wrong: afferent/efferent count files; farness/harmonic/residual are over INCOMING distances, harmonic is a sum, "residual" removes nothing.
- E22 Go `internal` rule shows "Kept ✓" on Java/TS codebases.
- U2 Overview "Direct dependencies 10,370" = import references (2,603 component dependencies).

## Misleading or inconsistent
U3/U4 Overview labels; U5/U11 two git universes (Overview/Activity: current files, 114 contributors;
Authors: all history, 141); E19 directory vs component aggregation; E20 directories drop files without
snippets; E21 two co-change definitions; E14 TS "abstract types" = interfaces; U14 Units landing states
graph coverage as fact; U17 top dependents include type-only; latent type-only differences (U9, U21).

## UI layout (verified by screenshot)
At 960–1100px (min window 960; default 1280): Units flow gets ~412px, names cut to ~8 chars, anomaly
chips wrap to 3 lines while the empty inspector keeps its width; author names cut to ~5 chars; author
stat tiles truncate ("+492,5…"). File "Imported by" paths truncate from the right, so rows look identical.
Label overlaps in the Java neighbourhood and Hotspots file grain. Fine at 1280+.

## Corrected or withdrawn
- Pass 1 guessed C# shares Java's same-package gap: it doesn't (3% missing).
- "1,622 cycles not shortest" (pass 1, mid-run): my check misread the view. Cycles are correct.
- "Forward bands broken": my query missed an `<aside>`.
- Pass 2 mid-run: opening a component corrupts other edges — only C# weights.
- Sonnet's "Units flow needs double-click" / "Hotspots click zooms": its clicks missed; single click works.

---

# Fix rounds — 2026-09-23

Four rounds of fix → rescan → re-audit → UI check. Every round rescanned the six codebases (plus
Exposed, Kotlin, from round 4) with the working-tree engine through `appscan.main.go`, re-ran the audit
scripts, re-ran the Units view's real pipeline on the snapshots, and toured the UI (Sonnet drove the
browser; every claim it reported was recomputed from the snapshot or re-checked in the browser here).
Nothing is committed.

## Round 1 — the findings above
Fixed: E1 React detection, E3 `java_class` as a codebase metric, E5 hop counts, E7/E30 metric
descriptions, E8 `@interface` units, E9 Java same-package/wildcard refs, E10 Go refs, E11/E29 module-level
usage (module units), E12 age, E16/U12 deleted-file component, E18/U13 unscored health, E22 Go rule
scope, E23 `.git` walked, E25/E26 all-branch git history, E27 sweeping commits, E28 vendored/minified JS,
E31 author identity, U1 never-imported, U2 Overview labels, U8/U25 Connections file edges, U16
Unclassified lane, U18 file imports page, U20 group sizes, U22 hop cap, U24 crowded, plus nested
`.gitignore` handling. Verified on rescans: unit edges gin 10 → 2,639, Broadleaf 9,561 → 13,044, oscar
1,272 → 2,141, LibreChat 1,799 → 2,552; precision 600/600 in Java, Go, TS, Python.

## Round 2
| What was wrong | Evidence | Fixed |
|---|---|---|
| Every file at the repository root had no git history: the walker names it `./go.mod`, git `go.mod` | gin 52 of 129 files, oscar 29, nopCommerce 14, LibreChat 15, Broadleaf 5 — "files with no history" was entirely these | git paths named as walked; `git_repos` attributed files to "Unknown" and put `api-gateway/` in `api` — fixed too |
| `dir/*` in `.gitignore` pruned the directory, so `!dir/keep` never applied | nopCommerce lost 16 `App_Data/Index.htm` files git tracks | walker descends when a negation can apply |
| One person, two authors | Broadleaf: "Stanislav Fedorov"/"StanislavFedorov", "Marie Standeven"/"marieStandeven" (GitHub no-reply logins), " Voronkov Vitalii" with a leading space | logins joined to the full name they spell; names trimmed; full name preferred for display |
| Generated code read as the project's own | nopCommerce's NSwag `RateClient` (6,021 lines, 140 declarations) led "crowded"; 129 Django migrations | `complexity__files__generated`; no health reading; excluded from crowded; counted in never-imported's sentence |
| Vendored libraries not recognised by name | `spectrum.js` (79) and `purify.js` (65) led "crowded" | licence-and-release banner rule (0 hits in 4 own-code repos, all hits vendored in 3 others) |
| Vendored npm packages were components | 45 of nopCommerce's 710 components were `lib_npm` folders (moment's 137 locales) | third-party files belong to no component: 710 → 665 |
| Lanes read from the whole file's imports | gin: `context_test.go` imports a MongoDB package for one test; 347 test functions were "Stores & Clients" | engine `unit_uses` view; lanes placed by the imports a unit (or its members) uses: 347 → 5 |
| Connections, Suggest, component evidence and the cut plan counted type-only edges the component graph excludes | LibreChat: 1,091 drawn vs 1,070 runtime dependencies | all read `runtimeComponentEdges` |
| Shallow clones read as complete history | gin: 1 commit, 1 contributor, no warning | `git__shallow_clone` per repository; Overview captions say so |
| Ambiguous module names | "views is the largest", dozens of `index` / `__init__` | package entry files and shared names carry their directory |

## Round 3
| What was wrong | Evidence | Fixed |
|---|---|---|
| Every file counted one line too many | a one-line `views.py` read 2; empty files 1 | lines counted as `wc -l` does: oscar 995,443 → 994,052 |
| Empty files had NULL indentation and static complexity (0/0) | 88 oscar files | average of nothing is 0 |
| C# saw 55% of the dependencies its source names | `StandardPermission` used in 129 files, 0 edges; `IRepository<Product>` never an edge | member access, generics, properties, nullable/array, typeof, cast, `as`, patterns: recall 55.5% → 92.5%, precision 600/600; nopCommerce never-imported 1,717 → 1,054; helper snippets no longer stored (DB 377 → 357 MB) |
| File matrix `path_distance` guessed from import strings, varied between runs | import matched any directory suffix, in map order, then every file in it | distances from resolved unit edges: distance 1 = the unit graph exactly |
| Region lists did not keep their notes' promises | "Sorted by what each one declares" was sorted by imported-by with no such column; never-imported's "those importing nothing first" too | each region sets its order; Holds shown for crowded |
| My round-2 wiring of generated files never reached the findings | destructuring slip; typecheck blind to it | fixed, and the model's source is now typed so it cannot recur |

## Round 4
| What was wrong | Evidence | Fixed |
|---|---|---|
| Nested types named by package only, so distinct types merged | Exposed: `Town` inside six test classes was one unit; nopCommerce `ConvertFrom`/`ConvertTo` | Kotlin and C# name nested types through their enclosing type |
| Python methods after a nested class belonged to no class | oscar: `__str__`, `clean`, `generate_hash` were module functions, merged per file; every model's `Meta` a top-level type | owner is the innermost enclosing class; `Meta` owned by its model |
| "Declares N things" counted members in some languages only | gin `context.go` "declares 151" (one type and its methods); Java files never could | declared = not owned by another unit in the same file: oscar landing 6,044 → 1,789, top crowded = `catalogue/forms` 31 (source: 31) |
| Framework detection confident on a sliver | Exposed (an ORM) read as a Ktor app from 14 of 2,810 classes; 2,163 "repositories" | 10 strong classes also need a 1% share |
| A lane defined by "nothing references it" produced "never the other way round — a layer holding" | Exposed, Entry points | such lanes are not read for layering |
| Nested C# and Kotlin types counted as separate top-level declarations | nopCommerce `StandardPermission` "holds 11" (its permission groups) | a nested type is owned by its enclosing type, like Python's `Meta`: nopCommerce crowded 9 → 3 |
| "A ASP.NET Core codebase" | Units landing | article by the name's first letter |
| Kotlin grammar drops declarations after a parse error | Exposed: 94 of 2,363 types lost, 42 of 44 in `ColumnType.kt` | declarations the parser missed are recovered from the text with brace-matched spans: 94 → 6 (doc samples/test resources); Kotlin recall 97.0% → 98.3%, precision 793/800 |

## Verified at the end (db10)
Summary totals, coupling, reachability/paths, strongly connected groups, cycle set and co-change, git
totals against the repositories, directory line roll-ups, and code-smell formulas and roll-ups (file,
component, directory — every one exact on 7 codebases). Unit-edge precision: Java/Go/TS/Python 600/600,
C# 600/600, Kotlin 793/800. Recall: Java imports 10,350/10,350, TS 565/570, Python 176/180, C# 92.5%, Kotlin
98.3%. Summary average indentation recomputed from source: exact.

## Round 5 — Kotlin grammar, PHP, and what they turned up
| What | Evidence | Done |
|---|---|---|
| Kotlin grammar upgraded (fwcd 2024-10 → 2026-08 main) | Exposed: files with parse errors 225 → 17, ERROR nodes 2,096 → 28 | queries updated for the new tree (receiver is now a field) |
| Kotlin supertypes written as constructor calls were never read | every `object Users : Table()`; supertype markers on Exposed 334 → 3,532 | `constructor_invocation` and `explicit_delegation` forms read |
| Kotlin interfaces and sealed classes were not abstract | abstract types 106 → 192 | counted |
| Kotlin extension receivers read only when adjacent to the name | `fun <T> Column<T>.comment()` was a plain function merged with a method of the same name | receiver read from the declaration; a type-parameter receiver (`fun <T : Table> T.deleteWhere`) is no receiver |
| PHP read by four regular expressions: components only, no units, no references | Sylius | tree-sitter PHP pack: types, top-level functions, file-level module units; supertype, attribute and Doctrine docblock markers; references resolved as PHP does (fully qualified, `use` alias, own namespace). Sylius 14,431 unit edges, recall 100% (14,150/14,152; the 2 are my check's case folding), precision 600/600; SonataAdminBundle recall 100%, precision 600/600. Symfony detected; the Component→Bundle rule reports a real violation (line 16 of CatalogPromotionRepositoryInterface.php) |
| JS methods belonged to "the class above them" and anonymous default classes were not units | Sylius's Stimulus controllers (`export default class extends Controller`): 15 loose functions in one file led "crowded" | an anonymous default class is named for its file; a method belongs to the class that holds it |
| Files changed only in a merge had no history; merges reported nothing | 7 Sylius files | merges now report the files they changed relative to every parent (`--cc --raw`), never the branch work already counted: per-file commit counts equal `git log --full-history` non-merge commits plus such merges, 40/40 sampled on Broadleaf |
| Author names from Latin-1 commits stored as invalid UTF-8 | Sylius "Javier Gonz\xe1lez": the Authors page and the Overview's activity failed to read | undecodable author, email and message text is read as Latin-1, as git assumes; every snapshot's text is now valid UTF-8 |

Git semantics, for the record: a file's commits are every non-merge commit reachable from HEAD that touched
it -- including side-branch commits whose change a later merge discarded, which `git log -- <file>`
hides by default -- plus merges that changed it themselves.

The final UI pass (db10/db11) matched the recomputed values on every item checked: landing counts
(oscar 1,789, gin 1,112, Exposed 2,682), findings and their lists, Overview strips and captions, authors,
rules, cycles (532), hotspots callouts. A clean load shows no console errors.

Harness note: the browser harness serves one snapshot per port; after switching it the page must be
reloaded with a new query string. Round 2's tour changed only the `#` fragment and read a mix of two
snapshots, which is where its "stale Hotspots" and a mismatched Units finding came from.

## Known and left as is
- Same fully-qualified name in two build modules is one unit (Exposed's documentation samples; one test
  class in Broadleaf). Fixing it needs resolution scoped by build module.
- Kotlin: 17 of Exposed's 877 files still hold a parse error with the upgraded grammar; the text recovery
  pass stays for those.
- CommonJS destructuring with a rename (`{ primeFiles: primeCodeFiles } = require(...)`) is not a
  binding: 1 of LibreChat's 5 remaining TS misses.
- Unit/file edges include TS type-only references (15 of LibreChat's 2,552); component edges do not.
- Activity lists each commit's files and lines for files in the snapshot only, like the Overview totals.
- Hotspots' "largest by line count" can be a vendored file; it has no health reading and says "—".
