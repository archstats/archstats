package archstats

import (
	"io/ioutil"
)

type Directory interface {
	Measurable
	Files() []File
	FilesRecursive() []File

	SubDirectories() []Directory
	SubDirectoriesRecursive() []Directory
}

type directory struct {
	path           string
	files          []File
	subDirectories []Directory
	stats          Stats
}

func (dir *directory) Identity() string {
	return dir.path
}

func (dir *directory) Stats() Stats {
	if dir.stats == nil {
		var allStats []Stats
		for _, directory := range dir.subDirectories {
			allStats = append(allStats, directory.Stats())
		}
		files := dir.files
		for _, file := range files {
			allStats = append(allStats, file.Stats())
		}

		dir.stats = MergeStats(allStats)
	}
	return dir.stats
}

func (dir *directory) Files() []File {
	return dir.files
}

func (dir *directory) FilesRecursive() []File {
	var files []File

	for _, f := range dir.files {
		files = append(files, f)
	}

	for _, d := range dir.subDirectories {
		files = append(files, d.FilesRecursive()...)
	}
	return files
}

func (dir *directory) SubDirectories() []Directory {
	return dir.subDirectories
}
func (dir *directory) SubDirectoriesRecursive() []Directory {
	var dirs []Directory

	for _, subDirectory := range dir.SubDirectories() {
		dirs = append(dirs, subDirectory)
	}

	for _, d := range dir.subDirectories {
		dirs = append(dirs, d.SubDirectoriesRecursive()...)
	}
	return dirs
}

func processDirectory(dirAbsolutePath string, depth int, visitors []FileVisitor) Directory {
	dir := &directory{
		path: dirAbsolutePath,
	}

	files, err := ioutil.ReadDir(dirAbsolutePath)
	if err != nil {
		panic(err)
	}

	for _, entry := range files {
		path := dirAbsolutePath + entry.Name()
		if entry.IsDir() {
			path += "/"
			dir.subDirectories = append(dir.subDirectories, processDirectory(path, depth+1, visitors))
		} else {
			dir.files = append(dir.files, processFile(path, entry, visitors))
		}
	}
	if err != nil {
		return nil
	}
	return dir
}
