package archstats

import (
	"os"
)

type File interface {
	Measurable
}
type file struct {
	depth int
	path  string
	name  string
	stats Stats
}

type FileVisitor interface {
	VisitFile(*file, []byte)
}

func (f *file) Stats() Stats {
	return f.stats
}

func (f *file) Depth() int {
	return f.depth
}

func (f *file) Path() string {
	return f.path
}

func (f *file) Identity() string {
	return f.name
}
func (f *file) AddStat(stat string, amount int) {
	f.AddStats(Stats{stat: amount})
}

func (f *file) AddStats(stats Stats) {
	f.stats = f.stats.Merge(stats)
}

func processFile(absolutePath string, entry os.FileInfo, visitors []FileVisitor) *file {
	file := &file{
		path: absolutePath,
		name: entry.Name(),
	}
	//TODO cleanup error handling
	content, _ := os.ReadFile(absolutePath)

	for _, visitor := range visitors {
		visitor.VisitFile(file, content)
	}
	return file
}

type FileSizeStatGenerator struct{}

func (f *FileSizeStatGenerator) VisitFile(file *file, _ []byte) {
	fileInfo, _ := os.Stat(file.Path())
	file.AddStat("size_in_bytes", int(fileInfo.Size()))
}
