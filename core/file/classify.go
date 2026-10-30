package file

import (
	"path"
	"strings"
)

// Conventions are what one ecosystem's files say about themselves by name,
// place or header: that a file is a test, that a tool wrote it, that it is
// someone else's code carried in the repository, what part of building or
// running the software it describes.
//
// Core knows the questions and no ecosystem's answers (ADR 0025). Each
// language pack registers its conventions, and a domain extension registers
// the conventions of its domain: CI/CD names pipelines and containers,
// mobile names app manifests. Every field may be left empty.
type Conventions struct {
	// Name identifies the conventions. Registering a name twice registers it
	// once: Java and Kotlin both register the JVM's.
	Name string
	// IsTest reports whether a repo-relative path ("src/x", no "./") is test
	// code by this ecosystem's convention.
	IsTest func(path string) bool
	// IsGenerated reports whether a file's first 2 KiB carry this
	// ecosystem's generator marker.
	IsGenerated func(head []byte) bool
	// IsThirdParty reports whether a file is a distributed library by this
	// ecosystem's conventions. lowerPath is lower-cased with forward
	// slashes; content may be nil, and then only the path is judged.
	IsThirdParty func(lowerPath string, content []byte) bool
	// ThirdPartyDirs are where this ecosystem's package manager puts
	// dependencies: "node_modules/".
	ThirdPartyDirs []string
	// PackageDirExts are the file extensions of languages whose directories
	// are packages, so a directory named vendor or build inside the source
	// root is code (see InDirOutsideSourceRoot).
	PackageDirExts []string
	// SystemKind names what part of building, shipping or running the
	// software a file describes: one of the SystemKind constants, or "".
	SystemKind func(path string, content []byte) string
}

// A Classifier answers for a file from every registered convention, by the
// precedence core owns: third_party > generated > test > non_code >
// production.
type Classifier struct {
	conventions    []Conventions
	thirdPartyDirs []string
	packageDirExts map[string]bool
}

// NewClassifier holds these conventions, in this order.
func NewClassifier(conventions ...Conventions) *Classifier {
	c := &Classifier{thirdPartyDirs: append([]string(nil), genericThirdPartyDirs...), packageDirExts: map[string]bool{}}
	for _, conv := range conventions {
		c.Add(conv)
	}
	return c
}

// Add registers conventions after those already held, once per Name.
func (c *Classifier) Add(conv Conventions) {
	for _, held := range c.conventions {
		if held.Name == conv.Name {
			return
		}
	}
	c.conventions = append(c.conventions, conv)
	c.thirdPartyDirs = append(c.thirdPartyDirs, conv.ThirdPartyDirs...)
	for _, ext := range conv.PackageDirExts {
		c.packageDirExts[ext] = true
	}
}

// A Class is everything the conventions say about one file.
type Class struct {
	Role       string
	ThirdParty bool
	Generated  bool
	// Empty for third-party code: a vendored package's Dockerfile builds
	// nothing of this project's.
	SystemKind string
}

// Classify reads a file's path and content against every convention.
func (c *Classifier) Classify(filePath string, content []byte) Class {
	var out Class
	out.ThirdParty = c.IsThirdParty(filePath, content)
	out.Generated = !out.ThirdParty && c.IsGenerated(content)
	if !out.ThirdParty {
		out.SystemKind = c.SystemKind(filePath, content)
	}
	out.Role = c.Role(filePath, out.ThirdParty, out.Generated, out.SystemKind)
	return out
}

// Role is a file's one role, by precedence. A lockfile is text that is not
// code whatever its extension says.
func (c *Classifier) Role(filePath string, thirdParty, generated bool, systemKind string) string {
	switch {
	case thirdParty:
		return RoleThirdParty
	case generated:
		return RoleGenerated
	case c.IsTestPath(filePath):
		return RoleTest
	case IsNonCode(filePath), systemKind == SystemKindLockfile:
		return RoleNonCode
	default:
		return RoleProduction
	}
}

// IsTestPath reports whether a path is test code by the conventions that
// hold everywhere or by a registered ecosystem's. Paths are as the walker
// names them ("./x" at the root).
func (c *Classifier) IsTestPath(filePath string) bool {
	p := strings.TrimPrefix(filePath, "./")
	if genericTestDir.MatchString(strings.ToLower(p)) || genericTestName.MatchString(path.Base(p)) {
		return true
	}
	for _, conv := range c.conventions {
		if conv.IsTest != nil && conv.IsTest(p) {
			return true
		}
	}
	return false
}

// IsGenerated reports whether a file's header says a tool wrote it.
func (c *Classifier) IsGenerated(content []byte) bool {
	head := content
	if len(head) > 2048 {
		head = head[:2048]
	}
	if genericGenerated.Match(head) {
		return true
	}
	for _, conv := range c.conventions {
		if conv.IsGenerated != nil && conv.IsGenerated(head) {
			return true
		}
	}
	return false
}

// SystemKind is the first kind a convention names, in registration order.
func (c *Classifier) SystemKind(filePath string, content []byte) string {
	for _, conv := range c.conventions {
		if conv.SystemKind == nil {
			continue
		}
		if kind := conv.SystemKind(filePath, content); kind != "" {
			return kind
		}
	}
	return ""
}
