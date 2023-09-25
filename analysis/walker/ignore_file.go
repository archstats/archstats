package walker

import (
	"bufio"
	ignore "github.com/sabhiram/go-gitignore"
	"io/fs"
	"path/filepath"

	//"os"
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

func (ctx *ignoreContext) addIgnoreLines(fileSystem fs.FS, dirPath string, files []fs.DirEntry) {
	ctx.lines = append(ctx.lines, getIgnoreLinesInDir(fileSystem, dirPath, files)...)
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
	fullPath := filepath.Clean(path + string(filepath.Separator) + fileInfo.Name())
	file, err := fileSystem.Open(fullPath)
	if err != nil {
		panic(err)
	}

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
