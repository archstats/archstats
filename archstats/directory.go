package archstats

import (
	"io/ioutil"
)

type Directory interface {
	Measurable
	GetDescendantFiles() []File
	GetDescendantSubDirectories() []Directory
}

type directory struct {
	Path           string
	Files          []*file
	SubDirectories []*directory
	stats          Stats
}

func (dir *directory) Identity() string {
	return dir.Path
}

func (dir *directory) Stats() Stats {
	if dir.stats == nil {
		var allStats []Stats
		for _, directory := range dir.SubDirectories {
			allStats = append(allStats, directory.Stats())
		}
		files := dir.Files
		for _, file := range files {
			allStats = append(allStats, file.Stats())
		}

		dir.stats = MergeStats(allStats)
	}
	return dir.stats
}

func (dir *directory) GetDescendantFiles() []File {
	var files []File

	for _, file := range dir.Files {
		files = append(files, file)
	}

	for _, directory := range dir.SubDirectories {
		files = append(files, directory.GetDescendantFiles()...)
	}
	return files
}

func (dir *directory) GetDescendantSubDirectories() []Directory {
	var dirs []Directory

	for _, subDirectory := range dir.SubDirectories {
		dirs = append(dirs, subDirectory)
	}

	for _, directory := range dir.SubDirectories {
		dirs = append(dirs, directory.GetDescendantSubDirectories()...)
	}
	return dirs
}

func processDirectory(dirAbsolutePath string, depth int, visitors []FileVisitor) *directory {
	dir := &directory{
		Path: dirAbsolutePath,
	}

	files, err := ioutil.ReadDir(dirAbsolutePath)
	if err != nil {
		return nil
	}

	for _, entry := range files {
		path := dirAbsolutePath + entry.Name()
		if entry.IsDir() {
			path += "/"
			dir.SubDirectories = append(dir.SubDirectories, processDirectory(path, depth+1, visitors))
		} else {
			dir.Files = append(dir.Files, processFile(path, entry, visitors))
		}
	}
	if err != nil {
		return nil
	}
	return dir
}
