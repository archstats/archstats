package walker

import (
	"fmt"
	"github.com/archstats/archstats/core/file"
	"github.com/rs/zerolog/log"
	"io/fs"
	"os"
	filepath "path"
	"runtime"
	"sync"
	"time"
)

func WalkDirectoryConcurrently(dirAbsolutePath string, visitor func(file file.File)) error {
	dirFS := os.DirFS(dirAbsolutePath).(fs.ReadFileFS)

	allFiles, err := GetAllFiles(dirAbsolutePath)
	if err != nil {
		return err
	}
	WalkFiles(dirFS, allFiles, visitor)
	return nil
}

// maxWorkers returns a bounded concurrency limit: min(runtime.NumCPU()*2, 32).
func maxWorkers() int {
	n := runtime.NumCPU() * 2
	if n > 32 {
		return 32
	}
	return n
}

func WalkFiles(fileSystem fs.ReadFileFS, allFiles []PathToFile, visitor func(file file.File)) {
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, maxWorkers())
	wg.Add(len(allFiles))
	log.Debug().Msgf("Walking & reading %d files (max %d workers)", len(allFiles), maxWorkers())
	for _, theFile := range allFiles {
		sem <- struct{}{} // acquire slot
		go func(file PathToFile, group *sync.WaitGroup) {
			defer group.Done()
			defer func() { <-sem }() // release slot
			defer func() {
				// Safety net: a panicking visitor (e.g. an extension) must not kill the process.
				if r := recover(); r != nil {
					log.Warn().Msgf("Recovered from panic while processing %s, skipping file: %v", file.Path(), r)
				}
			}()
			start := time.Now()
			content, err := fileSystem.ReadFile(filepath.Clean(file.Path()))

			if err != nil {
				log.Warn().Err(err).Msgf("Skipping unreadable file %s", file.Path())
				return
			}

			if isBinary(content) {
				log.Debug().Msgf("Skipping binary file %s", file.Path())
				return
			}

			openedFile := &openedFile{
				path:    file.Path(),
				content: content,
			}

			visitor(openedFile)
			log.Debug().Msgf("Finished reading %s in %s", file.Path(), time.Since(start))
		}(theFile, wg)
	}
	wg.Wait()
	log.Debug().Msgf("Done reading %d files", len(allFiles))
}

func isBinary(content []byte) bool {
	limit := len(content)
	if limit > 8000 {
		limit = 8000
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

func GetAllFiles(dirAbsolutePath string) ([]PathToFile, error) {
	log.Debug().Msgf("Finding unignored files in %s", dirAbsolutePath)

	files, err := getAllFiles(os.DirFS(dirAbsolutePath).(fs.ReadDirFS), ".", 0, ignoreContext{})
	if err != nil {
		return nil, fmt.Errorf("error reading root directory %s: %w", dirAbsolutePath, err)
	}

	log.Debug().Msgf("Found %d files, %d files/directories ignored ", len(files.FoundFiles), len(files.IgnoredFiles))

	return files.FoundFiles, nil
}

type FileResults struct {
	FoundFiles   []PathToFile
	IgnoredFiles []string
}

func getAllFiles(fileSystem fs.ReadDirFS, dirAbsolutePath string, depth int, ignoreCtx ignoreContext) (*FileResults, error) {
	separator := "/"

	dirAbsolutePath = filepath.Clean(dirAbsolutePath)
	var foundFiles []PathToFile
	var ignoredFiles []string

	files, err := fileSystem.ReadDir(dirAbsolutePath)
	if err != nil {
		if depth == 0 {
			// An unreadable root is an error the caller should see.
			return nil, err
		}
		log.Warn().Err(err).Msgf("Skipping unreadable directory %s", dirAbsolutePath)
		return &FileResults{}, nil
	}

	ignoreCtx.addIgnoreLines(fileSystem, dirAbsolutePath, files)

	gitIgnore := ignoreCtx.getGitIgnore()
	for _, entry := range files {
		path := dirAbsolutePath + separator + entry.Name()

		if entry.IsDir() {
			path += separator
			if shouldIgnore(path, gitIgnore) {
				ignoredFiles = append(ignoredFiles, path)
				continue
			}
			allFiles, _ := getAllFiles(fileSystem, path, depth+1, ignoreCtx) // never errors at depth > 0
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
				log.Warn().Err(err).Msgf("Skipping file %s, error getting file info", path)
			}
		}
	}
	return &FileResults{
		FoundFiles:   foundFiles,
		IgnoredFiles: ignoredFiles,
	}, nil
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
