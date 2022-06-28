package archstats

import (
	"os"
)

type File interface {
	Measurable
	Stats() Stats
	Path() string
	AddStats(stats Stats)
	RecordSnippet(snippet Snippet)
}
type Snippet interface {
	Name() string
	Begin() int
	End() int
}
type file struct {
	depth    int
	path     string
	name     string
	snippets []Snippet
	stats    Stats
}

func (f *file) RecordSnippet(snippet Snippet) {
	f.snippets = append(f.snippets, snippet)
	f.RecordStat(snippet.Name(), 1)
}

type FileVisitor interface {
	VisitFile(File, []byte)
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

func (f *file) Name() string {
	return f.path
}
func (f *file) RecordStat(stat string, amount int) {
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

func (f *FileSizeStatGenerator) VisitFile(file File, _ []byte) {
	fileInfo, _ := os.Stat(file.Path())
	file.AddStats(Stats{"size_in_bytes": int(fileInfo.Size())})
}
