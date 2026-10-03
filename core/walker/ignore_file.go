package walker

import (
	"bufio"
	"github.com/rs/zerolog/log"
	ignore "github.com/sabhiram/go-gitignore"
	"io/fs"
	filepath "path"

	//"os"
	"strings"
)

var (
	ignoreFilesConst = [...]string{".gitignore", ".archstatsignore"}
)

// ignoreContext is every ignore file on the way down to a directory, each
// kept with the directory it sits in.
//
// Patterns in a nested .gitignore are relative to that file's directory. They
// used to be pooled and matched as if every file sat at the root, so an
// anchored pattern such as IntelliJ's `/workspace.xml` in `.idea/.gitignore`
// never matched `.idea/workspace.xml`, and IDE state was scanned as code.
type ignoreContext struct {
	layers []ignoreLayer
}

type ignoreLayer struct {
	base string
	gi   *ignore.GitIgnore
	// self is gi without the patterns that exclude only a directory's
	// contents (`dir/*`, `dir/**`): what matches here excludes the
	// directory itself, and git never looks inside it again.
	self *ignore.GitIgnore
	// The layer's negated patterns ("!keep.me"), without the "!".
	negations []string
}

func newLayer(base string, lines []string) ignoreLayer {
	var negations, self []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "!") {
			negations = append(negations, strings.TrimPrefix(t, "!"))
		}
		if !strings.HasPrefix(t, "!") && (strings.HasSuffix(t, "/*") || strings.HasSuffix(t, "/**")) {
			continue
		}
		self = append(self, l)
	}
	return ignoreLayer{base: base, gi: ignore.CompileIgnoreLines(lines...), self: ignore.CompileIgnoreLines(self...), negations: negations}
}

// rootContext starts a walk with the caller's patterns as a layer at the root.
func rootContext(patterns []string) ignoreContext {
	var lines []string
	for _, p := range patterns {
		if t := strings.TrimSpace(p); t != "" && !strings.HasPrefix(t, "#") {
			lines = append(lines, t)
		}
	}
	if len(lines) == 0 {
		return ignoreContext{}
	}
	return ignoreContext{layers: []ignoreLayer{newLayer(".", lines)}}
}

// Matcher reports whether a root-relative slash path matches the patterns,
// as the walk judged it: for filtering records of files (history) the walk
// never saw.
func Matcher(patterns []string) func(path string) bool {
	ctx := rootContext(patterns)
	if len(ctx.layers) == 0 {
		return func(string) bool { return false }
	}
	return func(path string) bool {
		p := strings.TrimPrefix(path, "./")
		if isCodeowners(p) {
			return false
		}
		if ctx.layers[0].gi.MatchesPath(p) {
			return true
		}
		// A file inside an ignored directory is ignored with it.
		for dir := filepath.Dir(p); dir != "." && dir != "/" && dir != ""; dir = filepath.Dir(dir) {
			if ctx.layers[0].gi.MatchesPath(dir + "/") {
				return true
			}
		}
		return false
	}
}

// isCodeowners: ownership files are never ignored; the app reads them.
func isCodeowners(path string) bool {
	return filepath.Base(path) == "CODEOWNERS"
}

// within returns the context for a directory: this one plus the ignore files
// the directory itself holds. A copy, so a sibling never sees them.
func (ctx ignoreContext) within(fileSystem fs.FS, dirPath string, files []fs.DirEntry) ignoreContext {
	lines := getIgnoreLinesInDir(fileSystem, dirPath, files)
	if len(lines) == 0 {
		return ctx
	}
	layers := make([]ignoreLayer, 0, len(ctx.layers)+1)
	layers = append(layers, ctx.layers...)
	layers = append(layers, newLayer(dirPath, lines))
	return ignoreContext{layers: layers}
}

// prunes reports whether a directory can be skipped whole.
//
// The ignore library matches `dir/*` against the directory itself, which git
// does not: git keeps the directory and ignores its contents one by one, which
// is what lets `!dir/Index.htm` bring a file back. Skipping the directory lost
// such files. Such a directory is therefore walked, and each file judged on
// its own, whenever a negation could apply somewhere inside it.
//
// A directory excluded itself (`node_modules`, `dist/`) is always pruned: git
// cannot re-include a file whose parent directory is excluded. Without this,
// one `!.env.example` anywhere had every node_modules walked file by file.
func (ctx ignoreContext) prunes(dirPath string) bool {
	if !ctx.ignores(dirPath) {
		return false
	}
	// The places CODEOWNERS lives are walked file by file, so it can be kept
	// while everything else in them stays ignored.
	if d := strings.TrimSuffix(strings.TrimPrefix(dirPath, "./"), "/"); d == ".github" || d == "docs" {
		return false
	}
	for _, l := range ctx.layers {
		if rel, ok := relativeTo(dirPath, l.base); ok && l.self.MatchesPath(rel) {
			return true
		}
	}
	for _, l := range ctx.layers {
		rel, ok := relativeTo(dirPath, l.base)
		if !ok {
			continue
		}
		rel = strings.TrimSuffix(rel, "/")
		for _, n := range l.negations {
			pattern := strings.TrimPrefix(n, "/")
			// No slash but a trailing one: the pattern can match at any depth.
			if !strings.Contains(strings.TrimSuffix(pattern, "/"), "/") {
				return false
			}
			if strings.HasPrefix(pattern, rel+"/") || strings.HasPrefix(pattern, "**") {
				return false
			}
		}
	}
	return true
}

// ignores reports whether any ignore file on the way down excludes the path,
// each matching it relative to its own directory.
func (ctx ignoreContext) ignores(path string) bool {
	if isIgnoreFile(path) {
		return true
	}
	if isCodeowners(path) {
		return false
	}
	for _, l := range ctx.layers {
		if rel, ok := relativeTo(path, l.base); ok && l.gi.MatchesPath(rel) {
			return true
		}
	}
	return false
}

func relativeTo(path, base string) (string, bool) {
	if base == "." || base == "" {
		return strings.TrimPrefix(path, "./"), true
	}
	prefix := strings.TrimSuffix(base, "/") + "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	return strings.TrimPrefix(path, prefix), true
}

func getIgnoreLinesInDir(fileSystem fs.FS, path string, entries []fs.DirEntry) []string {
	var globsToReturn []string

	for _, entry := range entries {
		shouldIgnore := isIgnoreFile(path + entry.Name())
		if shouldIgnore {
			globsToReturn = append(globsToReturn, getIgnoreLinesInFile(fileSystem, path, entry)...)
		}
	}
	return globsToReturn
}

func getIgnoreLinesInFile(fileSystem fs.FS, path string, fileInfo fs.DirEntry) []string {
	var globs []string
	fullPath := filepath.Clean(path + "/" + fileInfo.Name())
	file, err := fileSystem.Open(fullPath)
	if err != nil {
		log.Warn().Err(err).Msgf("Skipping unreadable ignore file %s", fullPath)
		return nil
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		globs = append(globs, scanner.Text())
	}
	return globs
}
func isIgnoreFile(path string) bool {
	for _, s := range ignoreFilesConst {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}
