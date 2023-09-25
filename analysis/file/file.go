package file

import (
	"io/fs"
)

type File interface {
	fs.FileInfo
	Path() string
	Content() []byte
}
