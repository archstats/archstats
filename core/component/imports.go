package component

import "github.com/archstats/archstats/core/file"

// Dirs is the scan's directories, arranged for an import resolver to look
// things up in.
type Dirs interface {
	// Has reports whether a directory holds a file of the scan.
	Has(dir string) bool
	// ShortestEndingIn is the shortest directory whose path ends, segment by
	// segment, in this one, or "" when none does.
	ShortestEndingIn(path string) string
}

// An ImportResolver says which component an import names, for a language
// whose imports mean more than their shape: Go's module paths, a JavaScript
// project's own package names and aliases, Python's leading dots. A language
// pack registers one; the component linker asks every resolver before it
// reads an import as a path (ADR 0025).
type ImportResolver interface {
	// Name identifies the resolver; registering a name twice registers it
	// once.
	Name() string
	// Prepare is called once per scan, before any import is resolved, with
	// the root and every file the walker found.
	Prepare(root string, files []*file.Results)
	// Resolve answers for one import written in fromFile, which sits in
	// fromDir. It returns the component and true when it knows the answer --
	// which may be to leave the import as written -- and false to leave the
	// import to the next resolver, and then to the linker's reading of paths.
	Resolve(fromFile, fromDir, imp string, dirs Dirs) (string, bool)
}
