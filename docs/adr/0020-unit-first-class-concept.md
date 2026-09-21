# 0020. Unit as a First-Class Concept

- **Status**: Accepted
- **Date**: 2026-09-21

## Context and Problem Statement

ADR 0007 made the component first-class. ADR 0004 made the snippet the unit of analysis. Between them there was nothing, and Java hid the gap.

A file and a component are entities: they have identity, they are indexed, edges run between them. A snippet is an observation — text of a type, at a position — with no identity, so two snippets naming the same class are unrelated as far as the model is concerned.

Java got away with it because of a coincidence: one file is one public type, and the path encodes the package and the name. That makes the file a fair stand-in for the type, so `java_full_class` could be a per-file stat registered with `LastRecordStatMerger`, and every view downstream could key on file. "Class" in archstats was a nickname for "file, in a language where files happen to be classes."

Measured on real projects, the stand-in fails three ways at once:

| Reality | Measured | What breaks |
|---|---|---|
| One type, many files | nopCommerce: 1,567 partial classes | last-record-wins keeps one file and discards the rest |
| Many types, one file | 141 nopCommerce files; normal in Kotlin, Scala, Go, TypeScript | all but one type is silently dropped |
| The unit is a function | gin: 1,344 functions to 178 types; LibreChat: 997 to 14 | nothing to attach identity to; a view of classes shows 14 things |
| A unit belongs to a type it does not live in | Exposed: 225 extension functions; gin: 430 methods | no place to record that `fun Table.selectAll()` belongs to `Table` |

## Decision Drivers

- **One consumer, any kind**: lanes, graphs and groups must not care whether they are looking at a type, a function or a module — otherwise the Go and TypeScript versions get written second, which in practice means never.
- **Groups are the slicing concept**: a group of units has to work everywhere, or "create group from selection" is available in some views and not others.
- **Nothing existing may move**: `java_full_class` remains, and remains the compatibility path for snapshots taken before units existed.

## Considered Options

- **A `types` table**: closest to what Java already has, and unable to represent gin, LibreChat or a partial class.
- **A `types` table now and a `functions` table later**: every view, reading and group model written twice.
- **One polymorphic unit**: a named, addressable thing carrying its kind.

## Decision Outcome

Chosen option: **one polymorphic unit**, in `core/unit`, as a third grain beside files and components. Snippets are unchanged and stay identity-less.

Polymorphic in three specific senses:

1. **Kind varies** — `type`, `function`, `module`.
2. **Files is plural** — the relationship to files is many-to-many, and folding by ID happens during aggregation.
3. **Owner is separate from location** — a receiver or an extension function's type, recorded without pretending the unit lives there.

Markers are deliberately untyped `{Source, Key, Value}` triples, because the role is not declared in the same place twice: an annotation in Java, the `.csproj` in C#, the *filename* in Django, the path in Next.js, a struct tag or comment directive in Go. `Source` says what kind of evidence it is rather than flattening them all into "annotation".

Units flow through `file.Results` exactly as stats and snippets do, so no new registration mechanism was needed. `Results` gains `Units`, `UnitsByKind`, `UnitsByFile` and `UnitByID`.

### Consequences

- **Good**: a C# partial class is one unit. nopCommerce produces 3,862 units from 3,650 files, 18 of them spanning several files — `Nop.Services.Installation.InstallationService` is one unit across four.
- **Good**: more units than files, so multi-type files are representable for the first time.
- **Good**: the lane classifier in the UI is already kind-agnostic, so a React component is a unit with `kind: function`, a path marker and imports.
- **Bad**: a third grain is a third thing to explain, and "component" versus "module" versus "unit" is now a real documentation burden.
- **Bad**: only Java and C# emit units so far. Every other ecosystem shows an empty `units` view, which is honest but not yet useful — and the ecosystems that most need it (Go, TypeScript) are the ones still missing.
