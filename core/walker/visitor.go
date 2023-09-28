package walker

import "github.com/RyanSusana/archstats/core/file"

type FileVisitor interface {
	Visit(file file.File)
}
