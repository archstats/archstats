package walker

import (
	"io/ioutil"
	"os"
	"strings"
)

type File interface {
	Path() string
	Info() os.FileInfo
	Content() []byte
}

func GetAllFiles(dirAbsolutePath string) []*FileDescription {
	return getAllFiles(dirAbsolutePath, 0, ignoreContext{})
}
func getAllFiles(dirAbsolutePath string, depth int, ignoreCtx ignoreContext) []*FileDescription {
	if !strings.HasSuffix(dirAbsolutePath, "/") {
		dirAbsolutePath = dirAbsolutePath + "/"
	}
	var snippets []*FileDescription

	files, _ := ioutil.ReadDir(dirAbsolutePath)
	ignoreCtx.Add(dirAbsolutePath, files)

	for _, entry := range files {
		path := dirAbsolutePath + entry.Name()
		if ignoreCtx.shouldIgnore(path) {
			continue
		}

		if entry.IsDir() {
			path += "/"
			snippets = append(snippets, getAllFiles(path, depth+1, ignoreCtx)...)
		} else {
			snippets = append(snippets, &FileDescription{
				Path: path,
				Info: entry,
			})
		}
	}
	return snippets
}

type FileDescription struct {
	Path string
	Info os.FileInfo
}
