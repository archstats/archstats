package walker

import (
	"github.com/archstats/archstats/core/file"
	"github.com/rs/zerolog/log"
	"io/fs"
	"os"
	filepath "path"
	"sync"
	"time"
)

func WalkDirectoryConcurrently(dirAbsolutePath string, visitor func(file file.File)) {
	dirFS := os.DirFS(dirAbsolutePath).(fs.ReadFileFS)

	allFiles := GetAllFiles(dirAbsolutePath)
	WalkFiles(dirFS, allFiles, visitor)
}

func WalkFiles(fileSystem fs.ReadFileFS, allFiles []PathToFile, visitor func(file file.File)) {
	wg := &sync.WaitGroup{}
	wg.Add(len(allFiles))
	log.Debug().Msgf("Walking & reading %d files", len(allFiles))
	for _, theFile := range allFiles {
		go func(file PathToFile, group *sync.WaitGroup) {
			start := time.Now()
			//TODO cleanup error handling
			content, err := fileSystem.ReadFile(filepath.Clean(file.Path()))

			if err != nil {
				log.Error().Err(err).Msgf("Error reading file %s", file.Path())
				panic(err)
			}
			openedFile := &openedFile{
				path:    file.Path(),
				content: content,
			}

			visitor(openedFile)
			log.Debug().Msgf("Finished reading %s in %s", file.Path(), time.Since(start))
			group.Done()
		}(theFile, wg)
	}
	wg.Wait()
	log.Debug().Msgf("Done reading %d files", len(allFiles))
}

func GetAllFiles(dirAbsolutePath string) []PathToFile {
	log.Debug().Msgf("Finding unignored files in %s", dirAbsolutePath)

	files := getAllFiles(os.DirFS(dirAbsolutePath).(fs.ReadDirFS), ".", 0, ignoreContext{})

	log.Debug().Msgf("Found %d files, %d files/directories ignored ", len(files.FoundFiles), len(files.IgnoredFiles))

	return files.FoundFiles
}

type FileResults struct {
	FoundFiles   []PathToFile
	IgnoredFiles []string
}

func getAllFiles(fileSystem fs.ReadDirFS, dirAbsolutePath string, depth int, ignoreCtx ignoreContext) *FileResults {
	separator := "/"

	dirAbsolutePath = filepath.Clean(dirAbsolutePath)
	var foundFiles []PathToFile
	var ignoredFiles []string

	files, err := fileSystem.ReadDir(dirAbsolutePath)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error reading directory %s", dirAbsolutePath)
	}

	ignoreCtx.addIgnoreLines(fileSystem, dirAbsolutePath, files)

	gitIgnore := ignoreCtx.getGitIgnore()
	for _, entry := range files {
		path := dirAbsolutePath + separator + entry.Name()

		if entry.IsDir() {
			path += separator
			allFiles := getAllFiles(fileSystem, path, depth+1, ignoreCtx)
			foundFiles = append(foundFiles, allFiles.FoundFiles...)
			ignoredFiles = append(ignoredFiles, allFiles.IgnoredFiles...)
		} else {
			if shouldIgnore(path, gitIgnore) {
				ignoredFiles = append(ignoredFiles, path)
				continue
			}
			info, err := entry.Info()
			if err == nil {
				foundFiles = append(foundFiles, &pathToFile{
					path: path,
					info: info,
				})
			} else {
				log.Fatal().Err(err).Msgf("Error getting file info for %s", path)
			}
		}
	}
	return &FileResults{
		FoundFiles:   foundFiles,
		IgnoredFiles: ignoredFiles,
	}
}

type PathToFile interface {
	Path() string
	File() fs.FileInfo
}
type pathToFile struct {
	path string
	info fs.FileInfo
}

func (f *pathToFile) File() fs.FileInfo {
	return f.info
}

func (f *pathToFile) Path() string {
	return f.path
}
