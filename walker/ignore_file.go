package walker

import (
	"bufio"
	ignore "github.com/sabhiram/go-gitignore"
	"io/fs"
	"os"
	"strings"
)

var (
	ignoreFilesConst = [...]string{".gitignore", ".archstatsignore"}
)

type ignoreContext struct {
	lines []string
}

func (ctx *ignoreContext) getGitIgnore() *ignore.GitIgnore {
	return ignore.CompileIgnoreLines(ctx.lines...)
}

func (ctx *ignoreContext) addIgnoreLines(dirPath string, files []fs.FileInfo) {
	ctx.lines = append(ctx.lines, getIgnoreLinesInDir(dirPath, files)...)
}

func getIgnoreLinesInDir(path string, entries []fs.FileInfo) []string {
	var globsToReturn []string

	for _, entry := range entries {
		shouldIgnore := isIgnoreFile(path + entry.Name())
		if shouldIgnore {
			globsToReturn = append(globsToReturn, getIgnoreLinesInFile(path, entry)...)
		}
	}
	return globsToReturn
}

func getIgnoreLinesInFile(path string, fileInfo fs.FileInfo) []string {
	var globs []string
	file, _ := os.Open(path + fileInfo.Name())

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		globs = append(globs, scanner.Text())
	}
	return globs
}
func shouldIgnore(path string, gitIgnore *ignore.GitIgnore) bool {
	if isIgnoreFile(path) {
		return true
	}
	return gitIgnore.MatchesPath(path)
}
func isIgnoreFile(path string) bool {
	for _, s := range ignoreFilesConst {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}
