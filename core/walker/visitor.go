package walker

import "github.com/archstats/archstats/core/file"

type FileVisitor interface {
	Visit(file file.File)
}
