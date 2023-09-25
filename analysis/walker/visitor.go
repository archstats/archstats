package walker

import "github.com/RyanSusana/archstats/analysis/file"

type FileVisitor interface {
	Visit(file file.File)
}
