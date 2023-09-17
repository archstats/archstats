package file

import (
	"os"
)

type File interface {
	Description
	Content() []byte
}
type Description interface {
	Path() string
	Info() os.FileInfo
}
