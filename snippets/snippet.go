package snippets

import (
	"analyzer/walker"
	"os"
	"sync"
)

type Snippet struct {
	File      string
	Directory string
	Component string
	Type      string
	Begin     int
	End       int
	Value     string
}

type SnippetProvider interface {
	GetSnippetsFromFile(walker.File) []*Snippet
}

func GetSnippetsFromDirectory(rootPath string, visitors []SnippetProvider) []*Snippet {
	var toReturn []*Snippet
	allFiles := walker.GetAllFiles(rootPath)

	wg := &sync.WaitGroup{}
	lock := &sync.Mutex{}

	wg.Add(len(allFiles))
	for _, theFile := range allFiles {
		go func(file *walker.FileDescription, group *sync.WaitGroup) {
			snippets := getSnippetsFromFile(file.Path, file.Info, visitors)
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

func getSnippetsFromFile(absolutePath string, entry os.FileInfo, visitors []SnippetProvider) []*Snippet {
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
