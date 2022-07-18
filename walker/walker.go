package walker

import (
	"io/ioutil"
	"os"
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
	var snippets []*FileDescription

	files, _ := ioutil.ReadDir(dirAbsolutePath)
	ignoreCtx.Add(files)

	for _, entry := range files {
		if ignoreCtx.ShouldIgnore(entry) {
			continue
		}
		path := dirAbsolutePath + entry.Name()
		if entry.IsDir() {
			path += "/"
			snippets = append(snippets, getAllFiles(path, depth+1, ignoreCtx)...)
		} else {
			snippets = append(snippets, &FileDescription{
				AbsolutePath: path,
				Info:         entry,
			})
		}
	}
	return snippets
}

type FileDescription struct {
	AbsolutePath string
	Info         os.FileInfo
}
