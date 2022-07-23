package walker

import (
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

func WalkDirectoryConcurrently(dirAbsolutePath string, visitor func(file OpenedFile)) {
	allFiles := GetAllFiles(dirAbsolutePath)
	wg := &sync.WaitGroup{}
	wg.Add(len(allFiles))
	for _, theFile := range allFiles {
		go func(file FileDescription, group *sync.WaitGroup) {

			//TODO cleanup error handling
			content, _ := os.ReadFile(file.Path())
			openedFile := &absoluteFile{
				path:    file.Path(),
				info:    file.Info(),
				content: content,
			}

			visitor(openedFile)
			group.Done()
		}(theFile, wg)
	}
	wg.Wait()
}

func GetAllFiles(dirAbsolutePath string) []*fileDescription {
	return getAllFiles(dirAbsolutePath, 0, ignoreContext{})
}

type FileDescription interface {
	Path() string
	Info() os.FileInfo
}
type OpenedFile interface {
	FileDescription
	Content() []byte
}
type FileVisitor interface {
	Visit(file OpenedFile)
}

func getAllFiles(dirAbsolutePath string, depth int, ignoreCtx ignoreContext) []*fileDescription {
	if !strings.HasSuffix(dirAbsolutePath, "/") {
		dirAbsolutePath = dirAbsolutePath + "/"
	}
	var snippets []*fileDescription

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
			snippets = append(snippets, &fileDescription{
				path: path,
				info: entry,
			})
		}
	}
	return snippets
}

type fileDescription struct {
	path string
	info os.FileInfo
}

func (f *fileDescription) Path() string {
	return f.path
}

func (f *fileDescription) Info() os.FileInfo {
	return f.info
}

type absoluteFile struct {
	path    string
	info    os.FileInfo
	content []byte
}

func (a *absoluteFile) Content() []byte {
	return a.content
}

func (a *absoluteFile) Path() string {
	return a.path
}

func (a *absoluteFile) Info() os.FileInfo {
	return a.info
}
