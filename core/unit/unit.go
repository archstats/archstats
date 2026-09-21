// Package unit holds the named things in a codebase.
//
// archstats has had two grains. A file and a component are entities: they
// have identity, they are indexed, and edges run between them. A snippet is
// an observation -- "text of this type, at this position" -- with no identity
// at all, so two snippets naming the same class are unrelated as far as the
// model is concerned.
//
// Between them sat nothing, and Java hid the gap. In Java one file is one
// public type and the path encodes the package and the name, so the file is a
// fair stand-in for the type and `java_full_class` could be a per-file stat
// with a last-record-wins merger. "Class" in archstats was a nickname for
// "file, in a language where files happen to be classes."
//
// Measured on real projects, that stand-in fails three different ways:
//
//   - One type across many files. nopCommerce has 1,567 partial classes.
//   - Many types in one file. 141 nopCommerce files, and the normal way to
//     write Kotlin, Scala, Go and TypeScript.
//   - The unit is not a type at all. gin is 1,344 functions to 178 types;
//     LibreChat is 997 to 14. A view of classes shows fourteen things there.
//
// A Unit is therefore polymorphic in three specific senses: its Kind varies,
// its relationship to files is many-to-many, and what it belongs to is
// separate from where it lives -- Exposed has 225 extension functions, and
// `fun Table.selectAll()` belongs to Table while living somewhere else
// entirely.
package unit

import "sort"

// The kinds of thing a unit can be. Deliberately few: a kind exists when some
// ecosystem's architecture is made of it, not to mirror a grammar.
const (
	// A class, interface, record, struct, enum or trait.
	KindType = "type"
	// A function that stands on its own: a Go func, a React component, a
	// Kotlin top-level or extension function, a Python module-level def.
	KindFunction = "function"
	// A module as a unit in its own right, for ecosystems where that is what
	// architecture is made of -- a Go package, a Python module.
	KindModule = "module"
)

// Where a marker was read from. The role a unit plays is not declared in the
// same place twice: Java puts it in an annotation, C# in the project file,
// Django in the *filename*, Next.js in the path, Go in a struct tag or a
// comment directive. A marker is therefore untyped, and its Source says what
// kind of evidence it is rather than pretending they are all annotations.
const (
	SourceAnnotation = "annotation"
	SourceSupertype  = "supertype"
	SourceFilename   = "filename"
	SourcePath       = "path"
	SourceStructTag  = "struct_tag"
	SourceDirective  = "directive"
	SourceManifest   = "manifest"
)

// A Marker is one piece of evidence about what a unit is.
type Marker struct {
	Source string `json:"source"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// A Unit is a named, addressable thing in the code.
type Unit struct {
	// Stable across files and runs. Fully qualified where the ecosystem has
	// such a thing ("Nop.Core.Domain.Customer"), otherwise qualified by
	// where it lives ("lib/router.go#Engine.ServeHTTP").
	ID string `json:"id"`
	// One of the Kind constants.
	Kind string `json:"kind"`
	// What a person calls it, unqualified.
	Name string `json:"name"`
	// Every file this unit is declared in. Plural because a C# partial class
	// is one type spread across several, and folding them into one is the
	// whole reason units exist.
	Files []string `json:"files"`
	// Filled in by the engine once components are resolved.
	Component string `json:"component"`
	Module    string `json:"module"`
	// The unit this one belongs to without necessarily living in it: a Go
	// method's receiver, a Kotlin extension function's type. Empty for a
	// unit that belongs to nothing.
	Owner string `json:"owner"`

	Markers []Marker `json:"markers"`
}

// Merge folds units that are the same thing seen in different files.
//
// nopCommerce declares 1,567 partial classes; each file holding one produces
// a unit, and all of them are one type. Files and markers union, and the
// first non-empty value wins for everything else, so a partial that carries
// the annotations does not lose them to one that carries none.
//
// The result is ordered by ID, so a view built from it is stable between
// runs and a diff of two exports means something.
func Merge(units []*Unit) []*Unit {
	byID := make(map[string]*Unit, len(units))
	var order []string
	for _, u := range units {
		if u == nil || u.ID == "" {
			continue
		}
		existing, seen := byID[u.ID]
		if !seen {
			clone := *u
			clone.Files = append([]string(nil), u.Files...)
			clone.Markers = append([]Marker(nil), u.Markers...)
			byID[u.ID] = &clone
			order = append(order, u.ID)
			continue
		}
		for _, f := range u.Files {
			existing.Files = appendUniqueString(existing.Files, f)
		}
		for _, m := range u.Markers {
			existing.Markers = appendUniqueMarker(existing.Markers, m)
		}
		if existing.Owner == "" {
			existing.Owner = u.Owner
		}
		if existing.Name == "" {
			existing.Name = u.Name
		}
	}
	sort.Strings(order)
	out := make([]*Unit, 0, len(order))
	for _, id := range order {
		u := byID[id]
		sort.Strings(u.Files)
		out = append(out, u)
	}
	return out
}

// HasMarker reports whether a unit carries a marker with this key, whatever
// the evidence it came from. A consumer asking "is this a controller" should
// not have to know whether the answer was an annotation or a filename.
func (u *Unit) HasMarker(key string) bool {
	for _, m := range u.Markers {
		if m.Key == key {
			return true
		}
	}
	return false
}

// MarkerValues returns every value recorded for a key.
func (u *Unit) MarkerValues(key string) []string {
	var out []string
	for _, m := range u.Markers {
		if m.Key == key {
			out = append(out, m.Value)
		}
	}
	return out
}

func appendUniqueString(list []string, v string) []string {
	if v == "" {
		return list
	}
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

func appendUniqueMarker(list []Marker, m Marker) []Marker {
	if m.Key == "" {
		return list
	}
	for _, existing := range list {
		if existing == m {
			return list
		}
	}
	return append(list, m)
}
