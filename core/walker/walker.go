package walker

import (
	"fmt"
	"github.com/archstats/archstats/core/file"
	"github.com/rs/zerolog/log"
	"io/fs"
	"os"
	filepath "path"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Options adjust a walk beyond what the tree's own ignore files say.
type Options struct {
	// IgnorePatterns are gitignore-style patterns applied from the root, as
	// if one more ignore file sat there: a workspace's own exclusions.
	IgnorePatterns []string
}

func merged(opts []Options) Options {
	var out Options
	for _, o := range opts {
		out.IgnorePatterns = append(out.IgnorePatterns, o.IgnorePatterns...)
	}
	return out
}

func WalkDirectoryConcurrently(dirAbsolutePath string, visitor func(file file.File), opts ...Options) error {
	_, err := WalkAndReport(dirAbsolutePath, visitor, opts...)
	return err
}

// WalkAndReport walks every unignored file and returns what it left out.
func WalkAndReport(dirAbsolutePath string, visitor func(file file.File), opts ...Options) (*Ignored, error) {
	dirFS := os.DirFS(dirAbsolutePath).(fs.ReadFileFS)
	files, err := Scan(dirAbsolutePath, opts...)
	if err != nil {
		return nil, err
	}
	WalkFiles(dirFS, files.FoundFiles, visitor)
	return files.Ignored(), nil
}

// Ignored is what a scan left out: whole directories it never entered and
// single files it skipped, the shallowest of either first.
type Ignored struct {
	Files int
	Dirs  int
	// Top is the shallowest ignored paths, directories with a trailing "/":
	// node_modules/ is one line, not forty thousand.
	Top []string
}

// Ignored summarises what was left out.
func (r *FileResults) Ignored() *Ignored {
	out := &Ignored{}
	for _, p := range r.IgnoredFiles {
		if strings.HasSuffix(p, "/") {
			out.Dirs++
		} else {
			out.Files++
		}
	}
	top := append([]string(nil), r.IgnoredFiles...)
	sort.SliceStable(top, func(i, j int) bool {
		di, dj := strings.Count(strings.TrimSuffix(top[i], "/"), "/"), strings.Count(strings.TrimSuffix(top[j], "/"), "/")
		if di != dj {
			return di < dj
		}
		return top[i] < top[j]
	})
	if len(top) > 20 {
		top = top[:20]
	}
	for i, p := range top {
		top[i] = strings.TrimPrefix(p, "./")
	}
	out.Top = top
	return out
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

func GetAllFiles(dirAbsolutePath string, opts ...Options) ([]PathToFile, error) {
	files, err := Scan(dirAbsolutePath, opts...)
	if err != nil {
		return nil, err
	}
	return files.FoundFiles, nil
}

// Scan lists the files to analyse and everything it ignored.
func Scan(dirAbsolutePath string, opts ...Options) (*FileResults, error) {
	log.Debug().Msgf("Finding unignored files in %s", dirAbsolutePath)
	files, err := getAllFiles(os.DirFS(dirAbsolutePath).(fs.ReadDirFS), ".", 0, rootContext(merged(opts).IgnorePatterns))
	if err != nil {
		return nil, fmt.Errorf("error reading root directory %s: %w", dirAbsolutePath, err)
	}
	log.Debug().Msgf("Found %d files, %d files/directories ignored ", len(files.FoundFiles), len(files.IgnoredFiles))
	return files, nil
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

	ignoreCtx = ignoreCtx.within(fileSystem, dirAbsolutePath, files)

	for _, entry := range files {
		path := dirAbsolutePath + separator + entry.Name()

		// Version-control metadata is never source, and no ignore file lists
		// it: git does not track .git, so .gitignore has no reason to name it.
		// Walking it counted packed-refs and hook samples as code.
		if isVCSMetadata(entry.Name()) {
			if entry.IsDir() {
				path += separator
			}
			ignoredFiles = append(ignoredFiles, path)
			continue
		}

		if entry.IsDir() {
			path += separator
			if ignoreCtx.prunes(path) {
				ignoredFiles = append(ignoredFiles, path)
				continue
			}
			allFiles, _ := getAllFiles(fileSystem, path, depth+1, ignoreCtx) // never errors at depth > 0
			foundFiles = append(foundFiles, allFiles.FoundFiles...)
			ignoredFiles = append(ignoredFiles, allFiles.IgnoredFiles...)
		} else {
			if ignoreCtx.ignores(path) {
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

// isVCSMetadata reports whether a directory entry is a version-control
// system's own bookkeeping. `.git` is also a plain file in a worktree or a
// submodule, pointing at the real repository, so both forms are skipped.
func isVCSMetadata(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", ".bzr", "_darcs", ".jj":
		return true
	}
	return false
}
