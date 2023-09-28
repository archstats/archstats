package walker

import (
	"github.com/RyanSusana/archstats/core/file"
	"io/fs"
	"os"
	"time"
)

type openedFile struct {
	path    string
	info    os.FileInfo
	content []byte
}

func (a *openedFile) typeAssertion() file.File {
	return a
}
func (a *openedFile) Name() string {
	return a.info.Name()
}

func (a *openedFile) Size() int64 {
	return a.info.Size()
}

func (a *openedFile) Mode() fs.FileMode {
	return a.info.Mode()
}

func (a *openedFile) ModTime() time.Time {
	return a.info.ModTime()
}

func (a *openedFile) IsDir() bool {
	return a.info.IsDir()
}

func (a *openedFile) Sys() any {
	return a.info.Sys()
}

func (a *openedFile) Content() []byte {
	return a.content
}

func (a *openedFile) Path() string {
	return a.path
}

func (a *openedFile) Info() os.FileInfo {
	return a.info
}
