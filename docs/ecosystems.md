# Ecosystems

What each supported ecosystem actually looks like, measured on a real project
rather than assumed. Everything here was counted before the code that reads it
was written, and the numbers are why the code is shaped the way it is.

Java is the ecosystem archstats knew first, and it set the shape of everything
built on top of it: a *type*, in a *file*, carrying *annotations*, with
*extends*, *implements*, *fields* and *methods*. That description is true of
Java, mostly true of C# and Kotlin, and substantially false of the rest.

## The unit of architecture

| Project | Ecosystem | Types | Functions outside types |
|---|---|---|---|
| BroadleafCommerce | Java | 2,871 | — |
| nopCommerce | C# | ~3,650 files | — |
| Sylius | PHP | 3,997 | — |
| django-oscar | Python | 837 | 191 |
| Exposed | Kotlin | 1,319 | 696 (471 top-level, 225 extension) |
| gin | Go | 178 | 1,344 |
| archstats | Go | 106 | 556 |
| LibreChat | TypeScript | 14 | 997 |

A view of classes shows fourteen things in LibreChat, out of about a thousand.

## Where the role is declared

Broadleaf's most common annotations are `@Component` (368), `@Service` (280),
`@Entity` (157) and `@Repository` (67). The role is declared, in an
annotation, on a type. Nowhere else does that hold.

| Ecosystem | Where the role lives | Evidence |
|---|---|---|
| Java | annotation on the type | as above |
| C# | the `.csproj` and folder | attributes are display and serialization: `NopResourceDisplayName` 2,609, `JsonProperty` 1,064 |
| PHP | interfaces and composer package | 344 interfaces to 663 classes in Sylius's `Component`; the top attributes are Behat test steps (`#[Then` 2,762) |
| Python | **the filename** | django-oscar: `apps.py` 29, `views.py` 25, `models.py` 24, `forms.py` 20, `admin.py` 14 — against 39 architectural decorators in the whole repository |
| Kotlin | gradle module and naming | the top annotations are `@Suppress`, `@JvmName`, `@OptIn` — none of them roles |
| Go | package and function signature | no annotations at all; 316 `HandlerFunc` in gin, 150 `form:` struct tags |
| TypeScript | filename and path | decorators exist only in Angular and NestJS |

For Django, the filename *is* the annotation.

## Where the module boundary is declared

Read by `core/module` (ADR 0017). Counts are what the readers find today.

| Ecosystem | Manifest | Found |
|---|---|---|
| C# | `.csproj` | nopCommerce: 40 — `Nop.Core`, `Nop.Services`, and thirty `Nop.Plugin.*` |
| PHP | `composer.json` | Sylius: 60, split into `Component` and `Bundle` |
| Kotlin / Java | `build.gradle.kts`, `pom.xml` | Exposed: 50 gradle; Broadleaf: 13 maven |
| TypeScript | `package.json` workspaces | LibreChat: `api`, `client`, `packages/*` |
| Python | `apps.py` | django-oscar: 29 apps in `src`, 39 including tests |
| Go | `go.mod`, `internal/` | module path, and a boundary the compiler enforces |

## Edges that are not ordinary imports

See ADR 0018 for kinds and ADR 0019 for what cannot be resolved.

- **Erased.** TypeScript's `import type` is a dependency on a shape and none
  at runtime. LibreChat: 681 of 5,100 imports, 293 of 2,862 component edges.
- **Resolved from a string.** django-oscar: 746 `get_class`/`get_model` calls
  against 1,645 static imports. 664 references now resolve into real edges
  and 17 lookups are reported as unresolvable; before this, all of it was
  simply absent.
- **Composed, not referenced.** Go embedding: gin embeds `RouterGroup` and
  `ResponseWriter`. Kotlin delegation is the same shape.
- **Undeclared.** Go interface satisfaction is written nowhere and can only be
  computed by matching method sets.
- **Declared only in a manifest.** A `.csproj` `ProjectReference` is how a
  .NET plugin reaches core, with no `using` anywhere.
- **Laundered.** LibreChat has 77 barrel files; each `index.ts` hides the real
  source of a dependency.

## The rule that matters

Each ecosystem has one or two statements that decide whether its architecture
is intact. Those that read only the module map and the edge list are shipped
in `extensions/rules`; the rest need the unit grain and are not built yet.

| Ecosystem | Rule | Shipped |
|---|---|---|
| PHP / Symfony | `Component` must never depend on `Bundle` | yes |
| C# / .NET | core must never depend on a plugin | yes |
| Go | nothing outside a module may import its `internal/` | yes |
| Python / Django | app isolation, including through `get_class` | not yet |
| Kotlin | gradle module boundaries and `@InternalApi` | not yet |
| TypeScript | workspace boundaries, runtime and type-only kept apart | not yet |
| Java / Spring | layering between controller, service and repository | not yet — needs units |

Run against the projects measured here, the shipped rules report no violations
in nopCommerce and one in Sylius:
`Sylius\Component\Promotion\Repository\CatalogPromotionRepositoryInterface`
imports `Sylius\Bundle\PromotionBundle\Criteria\CriteriaInterface`, which is
the framework-agnostic domain reaching into the framework glue.

## Known gaps

- **Scala** is still read by regular expression only, and has no fixture in
  the real-project suite.
- **Marker noise.** Sylius has 8,028 PHP attributes and 5,898 of them are
  Behat test steps. Anything that reads markers will need per-ecosystem
  filtering or it will drown.
- **Units exist for Java and C# only.** Every other ecosystem renders an
  empty `units` view, and the two that most need it -- Go and TypeScript --
  are among the missing.

## Not a gap, though it looks like one

**Root-level source files are analysed.** The JavaScript, TypeScript and
Python packs glob `**/*.ts` and friends, and those patterns genuinely do not
match a bare `index.ts`, which makes it look as though every root-level file
in those ecosystems is silently skipped. They are not: the walker emits
root-level files as `./index.ts`, with a separator, and the patterns match
that.

The behaviour is correct and rests on two things agreeing that nothing states
out loud -- the path format the walker produces and the shape of every pack's
glob. Change either and root-level files disappear with no error anywhere, so
`e2eTest/rootlevel_e2e_test.go` pins it.
