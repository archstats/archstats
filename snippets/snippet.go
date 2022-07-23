package snippets

import (
	"os"
	"sync"
)

type Snippet struct {
	File      string `json:"file"`
	Directory string `json:"directory"`
	Component string `json:"component"`
	Type      string `json:"type"`
	Begin     int    `json:"begin"`
	End       int    `json:"end"`
	Value     string `json:"value"`
}
type FileDescription interface {
	Path() string
	Info() os.FileInfo
}
type File interface {
	FileDescription
	Content() []byte
}

type SnippetProvider interface {
	GetSnippetsFromFile(File) []*Snippet
}

func GetSnippetsFromDirectory(allFiles []FileDescription, visitors []SnippetProvider) []*Snippet {
	var toReturn []*Snippet

	wg := &sync.WaitGroup{}
	lock := &sync.Mutex{}

	wg.Add(len(allFiles))
	for _, theFile := range allFiles {
		go func(file FileDescription, group *sync.WaitGroup) {
			snippets := getSnippetsFromFile(file.Path(), file.Info(), visitors)
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
