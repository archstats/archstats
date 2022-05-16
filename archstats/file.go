package archstats

import (
	"os"
)

type File struct {
	depth int
	path  string
	name  string
	stats Stats
}

type FileVisitor interface {
	VisitFile(*File, []byte)
}

func (f *File) Stats() Stats {
	return f.stats
}

func (f *File) Depth() int {
	return f.depth
}

func (f *File) Path() string {
	return f.path
}

func (f *File) Identity() string {
	return f.name
}
func (f *File) AddStat(stat string, amount int) {
	f.AddStats(Stats{stat: amount})
}

func (f *File) AddStats(stats Stats) {
	f.stats = f.stats.Merge(stats)
}

func processFile(absolutePath string, entry os.FileInfo, visitors []FileVisitor) *File {
	file := &File{
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

func (f *FileSizeStatGenerator) VisitFile(file *File, _ []byte) {
	fileInfo, _ := os.Stat(file.Path())
	file.AddStat("size_in_bytes", int(fileInfo.Size()))
}
