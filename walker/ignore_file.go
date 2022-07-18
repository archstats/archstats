package walker

import (
	"bufio"
	"github.com/gobwas/glob"
	"io/fs"
	"os"
	"strings"
)

var (
	ignoreFilesConst = [...]string{".gitignore", ".archstatsignore"}
)

// TODO test ignore
type ignoreContext struct {
	globs []glob.Glob
}

func (ctx *ignoreContext) Add(path string, files []fs.FileInfo) {
	ctx.globs = append(findGlobsInDir(path, files))
}

func findGlobsInDir(path string, entries []fs.FileInfo) []glob.Glob {
	var globsToReturn []glob.Glob

	for _, entry := range entries {
		shouldIgnore := isIgnoreFile(path + entry.Name())
		if shouldIgnore {
			globsToReturn = append(globsToReturn, getGlobsFromFile(path, entry)...)
		}
	}
	return globsToReturn
}

func getGlobsFromFile(path string, fileInfo fs.FileInfo) []glob.Glob {
	var globs []glob.Glob
	file, _ := os.Open(path + fileInfo.Name())

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		if g, err := glob.Compile(scanner.Text()); err == nil {
			globs = append(globs, g)
		}
	}
	return globs
}
func (ctx *ignoreContext) shouldIgnore(path string) bool {
	for _, g := range ctx.globs {
		if g.Match(path) {
			return true
		}
	}
	return isIgnoreFile(path)
}
func isIgnoreFile(path string) bool {
	for _, s := range ignoreFilesConst {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}
