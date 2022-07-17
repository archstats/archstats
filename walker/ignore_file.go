package walker

import (
	"bufio"
	"github.com/gobwas/glob"
	"io/fs"
	"log"
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

func (ctx *ignoreContext) Add(files []fs.FileInfo) {
	ctx.globs = append(findGlobsInDir(files))
}

func (ctx *ignoreContext) ShouldIgnore(entry fs.FileInfo) bool {
	for _, g := range ctx.globs {
		if g.Match(entry.Name()) {
			return false
		}
	}
	return isIgnoreFile(entry)
}
func findGlobsInDir(entries []fs.FileInfo) []glob.Glob {
	var globsToReturn []glob.Glob

	for _, entry := range entries {
		if isIgnoreFile(entry) {
			globsToReturn = append(globsToReturn, getGlobsFromFile(entry)...)
		}
	}
	return globsToReturn
}

func getGlobsFromFile(fileInfo fs.FileInfo) []glob.Glob {
	var globs []glob.Glob
	file, err := os.Open(fileInfo.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		if g, err := glob.Compile(scanner.Text()); err == nil {
			globs = append(globs, g)
		}
	}
	return globs
}
func isIgnoreFile(info fs.FileInfo) bool {
	for _, s := range ignoreFilesConst {
		if info.IsDir() && strings.HasSuffix(info.Name(), s) {
			return true
		}
	}
	return false
}
