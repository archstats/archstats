package walker

import (
	"io/ioutil"
	"os"
	"strings"
)

func GetAllFiles(dirAbsolutePath string) []*FileDescription {
	return getAllFiles(dirAbsolutePath, 0, ignoreContext{})
}
func getAllFiles(dirAbsolutePath string, depth int, ignoreCtx ignoreContext) []*FileDescription {
	if !strings.HasSuffix(dirAbsolutePath, "/") {
		dirAbsolutePath = dirAbsolutePath + "/"
	}
	var snippets []*FileDescription

	files, _ := ioutil.ReadDir(dirAbsolutePath)
	ignoreCtx.addIgnoreLines(dirAbsolutePath, files)

	gitIgnore := ignoreCtx.getGitIgnore()
	for _, entry := range files {
		path := dirAbsolutePath + entry.Name()
		if shouldIgnore(path, gitIgnore) {
			continue
		}

		if entry.IsDir() {
			path += "/"
			snippets = append(snippets, getAllFiles(path, depth+1, ignoreCtx)...)
		} else {
			snippets = append(snippets, &FileDescription{
				path: path,
				info: entry,
			})
		}
	}
	return snippets
}

type FileDescription struct {
	path string
	info os.FileInfo
}

func (f *FileDescription) Path() string {
	return f.path
}

func (f *FileDescription) Info() os.FileInfo {
	return f.info
}
