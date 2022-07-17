package walker

import (
	"fmt"
	"os"
	"sync"

	"io/ioutil"
)

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

func getSnippetsFromDirectory(rootPath string, visitors []SnippetProvider) []*Snippet {

	var toReturn []*Snippet
	allFiles := getDirectoryFiles(rootPath, 0, visitors, ignoreContext{})

	wg := &sync.WaitGroup{}
	lock := &sync.Mutex{}

	wg.Add(len(allFiles))
	for _, theFile := range allFiles {
		go func(file *fileDesc, group *sync.WaitGroup) {
			snippets := processFile(file.path, file.info, visitors)
			lock.Lock()
			for _, snippet := range snippets {
				toReturn = append(toReturn, snippet)
			}
			lock.Unlock()
			group.Done()
		}(theFile, wg)
	}
	wg.Wait()
	return toReturn
}

func getDirectoryFiles(dirAbsolutePath string, depth int, visitors []SnippetProvider, ignoreCtx ignoreContext) []*fileDesc {
	var snippets []*fileDesc

	files, _ := ioutil.ReadDir(dirAbsolutePath)
	ignoreCtx.Add(files)

	for _, entry := range files {
		if ignoreCtx.ShouldIgnore(entry) {
			fmt.Println("Ignoring: ", entry.Name())
			continue
		}
		path := dirAbsolutePath + entry.Name()
		if entry.IsDir() {
			path += "/"
			snippets = append(snippets, getDirectoryFiles(path, depth+1, visitors, ignoreCtx)...)
		} else {
			snippets = append(snippets, &fileDesc{
				path: path,
				info: entry,
			})
		}
	}
	return snippets
}

type fileDesc struct {
	path string
	info os.FileInfo
}

func processFile(absolutePath string, entry os.FileInfo, visitors []SnippetProvider) []*Snippet {
	var snippets []*Snippet
	//TODO cleanup error handling
	content, _ := os.ReadFile(absolutePath)
	file := &absoluteFile{
		path:    absolutePath,
		info:    entry,
		content: content,
	}

	for _, visitor := range visitors {
		snippets = append(snippets, visitor.GetSnippetsFromFile(file)...)
	}
	return snippets
}
