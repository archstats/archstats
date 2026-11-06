// Package grammar is tree-sitter-swift by alex-pinkus (MIT, see LICENSE),
// generated files from the 0.7.3-with-generated-files tag, commit 31d17fe.
//
// The generated C is not in git: `go generate` downloads it (fetch.go,
// sha256-pinned). Run it once after cloning:
//
//	go generate ./extensions/treesitter/swift/grammar
//
// The binding is ours rather than imported because that module's own
// binding test imports github.com/tree-sitter/tree-sitter-swift, a path that
// holds no such package, and `go mod tidy` loads the tests of every
// dependency: tidy failed in this repository and in every one that imports
// it, the desktop app among them, since `wails dev` runs tidy first.
//
// parser.c and scanner.c are compiled as separate files, not included into
// one, which also ends the TOKEN_COUNT redefinition warning the upstream
// binding printed on every build.
package grammar

//go:generate go run fetch.go

// #cgo CFLAGS: -std=c11 -fPIC
// #if !__has_include("parser.c")
// #error "Swift grammar sources are missing: run `go generate ./extensions/treesitter/swift/grammar`"
// #endif
// typedef struct TSLanguage TSLanguage;
// const TSLanguage *tree_sitter_swift(void);
import "C"

import "unsafe"

// Language is the tree-sitter language for Swift.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_swift())
}
