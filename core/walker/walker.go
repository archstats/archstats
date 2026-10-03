package walker

import (
	"bytes"
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
	"unicode/utf8"
)

// Options adjust a walk beyond what the tree's own ignore files say.
type Options struct {
	// IgnorePatterns are gitignore-style patterns applied from the root, as
	// if one more ignore file sat there: a workspace's own exclusions.
	IgnorePatterns []string
	// Claims reports whether a loaded language reads a path as source. Such
	// a file is binary only when a large share of it is not text: a TypeScript file
	// that joins keys with a literal NUL is still TypeScript, and dropping
	// it at its first NUL left its tests importing a file the snapshot did
	// not have.
	Claims func(path string) bool
}

func merged(opts []Options) Options {
	var out Options
	var claims []func(string) bool
	for _, o := range opts {
		out.IgnorePatterns = append(out.IgnorePatterns, o.IgnorePatterns...)
		if o.Claims != nil {
			claims = append(claims, o.Claims)
		}
	}
	if len(claims) > 0 {
		out.Claims = func(path string) bool {
			for _, c := range claims {
				if c(path) {
					return true
				}
			}
			return false
		}
	}
	return out
}

func WalkDirectoryConcurrently(dirAbsolutePath string, visitor func(file file.File), opts ...Options) error {
	_, err := WalkAndReport(dirAbsolutePath, visitor, opts...)
	return err
}

// WalkAndReport walks every unignored file and returns what it left out.
func WalkAndReport(dirAbsolutePath string, visitor func(file file.File), opts ...Options) (*Report, error) {
	dirFS := os.DirFS(dirAbsolutePath).(fs.ReadFileFS)
	files, err := Scan(dirAbsolutePath, opts...)
	if err != nil {
		return nil, err
	}
	skipped := walkFiles(dirFS, files.FoundFiles, visitor, merged(opts).Claims)
	return &Report{
		Ignored: files.Ignored(),
		Skipped: files.skipped(skipped),
	}, nil
}

// Why a path was left out of a scan.
const (
	// SkipIgnored: an ignore file, a scan-level pattern or version-control
	// metadata excluded it. A directory is one entry; its files are not
	// listed, since the walker never entered it.
	SkipIgnored = "ignored"
	// SkipBinary: the file was read and is not text.
	SkipBinary = "binary"
	// SkipUnreadable: the file or directory could not be read.
	SkipUnreadable = "unreadable"
	// SkipFailed: the file was read but analysing it failed.
	SkipFailed = "failed"
)

// Skipped is one path a scan left out, and why. Directories end in "/".
type Skipped struct {
	Path   string
	Reason string
	// Detail is the specifics, when there are any: the read error, where
	// the NUL byte was.
	Detail string
}

// Report is what a walk left out.
type Report struct {
	Ignored *Ignored
	// Skipped is every path left out, sorted by path.
	Skipped []Skipped
}

// skipped lists every path the listing and the reading left out, sorted.
func (r *FileResults) skipped(read []Skipped) []Skipped {
	out := make([]Skipped, 0, len(r.IgnoredFiles)+len(r.UnreadablePaths)+len(read))
	for _, p := range r.IgnoredFiles {
		out = append(out, Skipped{Path: p, Reason: SkipIgnored})
	}
	out = append(out, r.UnreadablePaths...)
	out = append(out, read...)
	for i := range out {
		out[i].Path = strings.TrimPrefix(out[i].Path, "./")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
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
	walkFiles(fileSystem, allFiles, visitor, nil)
}

// walkFiles reads and visits every file, and returns the ones it skipped.
func walkFiles(fileSystem fs.ReadFileFS, allFiles []PathToFile, visitor func(file file.File), claims func(path string) bool) []Skipped {
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, maxWorkers())
	var skipped []Skipped
	var skippedLock sync.Mutex
	skip := func(path, reason, detail string) {
		skippedLock.Lock()
		skipped = append(skipped, Skipped{Path: path, Reason: reason, Detail: detail})
		skippedLock.Unlock()
	}
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
					skip(file.Path(), SkipFailed, fmt.Sprint(r))
				}
			}()
			start := time.Now()
			content, err := fileSystem.ReadFile(filepath.Clean(file.Path()))

			if err != nil {
				log.Warn().Err(err).Msgf("Skipping unreadable file %s", file.Path())
				skip(file.Path(), SkipUnreadable, err.Error())
				return
			}

			claimed := claims != nil && claims(file.Path())
			if why := binaryReason(content, claimed); why != "" {
				if claimed {
					// A language expected source here; say so above debug.
					log.Warn().Msgf("Skipping binary file %s: %s", file.Path(), why)
				} else {
					log.Debug().Msgf("Skipping binary file %s: %s", file.Path(), why)
				}
				skip(file.Path(), SkipBinary, why)
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
	return skipped
}

// sniffLen is how much of a file the binary checks look at, as git does.
const sniffLen = 8000

// maxNonText is the share of a claimed file's first bytes that may be
// control bytes or broken UTF-8 before it reads as binary. Source text,
// even with a NUL or a stray Latin-1 byte, is well under 1%; compressed or
// random bytes are over half.
const maxNonText = 0.3

// binaryReason says why content is binary, or "" when it is text. A file a
// loaded language claims is text unless a large share of it is not; any other file
// is binary at its first NUL, which is how git decides.
func binaryReason(content []byte, claimed bool) string {
	sample := content
	if len(sample) > sniffLen {
		sample = sample[:sniffLen]
	}
	if claimed {
		if share := nonTextShare(sample); share > maxNonText {
			return fmt.Sprintf("%.0f%% of the first %d bytes are not text", share*100, len(sample))
		}
		return ""
	}
	if i := bytes.IndexByte(sample, 0); i >= 0 {
		return fmt.Sprintf("NUL byte at offset %d", i)
	}
	return ""
}

func isBinary(content []byte) bool {
	return binaryReason(content, false) != ""
}

// nonTextShare is the share of sample's bytes that are control characters
// other than whitespace and escape, or not valid UTF-8. A multi-byte
// character cut off by the end of the sample is not counted against it.
func nonTextShare(sample []byte) float64 {
	if len(sample) == 0 {
		return 0
	}
	bad := 0
	for i := 0; i < len(sample); {
		if !utf8.FullRune(sample[i:]) {
			break
		}
		r, size := utf8.DecodeRune(sample[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			bad++
		case r < 0x20 && r != '\t' && r != '\n' && r != '\r' && r != '\f' && r != '\v' && r != 0x1b, r == 0x7f:
			bad += size
		}
		i += size
	}
	return float64(bad) / float64(len(sample))
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
	// UnreadablePaths are directories and files the listing could not read.
	UnreadablePaths []Skipped
}

func getAllFiles(fileSystem fs.ReadDirFS, dirAbsolutePath string, depth int, ignoreCtx ignoreContext) (*FileResults, error) {
	separator := "/"

	dirAbsolutePath = filepath.Clean(dirAbsolutePath)
	var foundFiles []PathToFile
	var ignoredFiles []string
	var unreadable []Skipped

	files, err := fileSystem.ReadDir(dirAbsolutePath)
	if err != nil {
		if depth == 0 {
			// An unreadable root is an error the caller should see.
			return nil, err
		}
		log.Warn().Err(err).Msgf("Skipping unreadable directory %s", dirAbsolutePath)
		return &FileResults{UnreadablePaths: []Skipped{{Path: dirAbsolutePath + "/", Reason: SkipUnreadable, Detail: err.Error()}}}, nil
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
			// Walked only because a negation might have kept something, and
			// nothing was: one line, as if it had been pruned. `dist/*` with
			// no dist/.gitkeep is not two hundred rows of bundle chunks.
			if len(allFiles.FoundFiles) == 0 && len(allFiles.UnreadablePaths) == 0 &&
				len(allFiles.IgnoredFiles) > 0 && ignoreCtx.ignores(path) {
				ignoredFiles = append(ignoredFiles, path)
				continue
			}
			foundFiles = append(foundFiles, allFiles.FoundFiles...)
			ignoredFiles = append(ignoredFiles, allFiles.IgnoredFiles...)
			unreadable = append(unreadable, allFiles.UnreadablePaths...)
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
				unreadable = append(unreadable, Skipped{Path: path, Reason: SkipUnreadable, Detail: err.Error()})
			}
		}
	}
	return &FileResults{
		FoundFiles:      foundFiles,
		IgnoredFiles:    ignoredFiles,
		UnreadablePaths: unreadable,
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
