package component

import "strings"

// DirIndex holds the scan's directories so that resolving an import is a
// lookup, not a pass over every file: on hibernate-orm those passes took 20
// seconds, one import at a time.
type DirIndex struct {
	dirs map[string]bool
	// bySuffix maps every whole-segment tail of a directory, the directory
	// itself included, to the shortest directory ending in it.
	bySuffix map[string]string
}

// NewDirIndex arranges the directories files sit in, by file.
func NewDirIndex(fileDirs map[string]string) *DirIndex {
	x := &DirIndex{dirs: map[string]bool{}, bySuffix: map[string]string{}}
	for _, dir := range fileDirs {
		if x.dirs[dir] {
			continue
		}
		x.dirs[dir] = true
		tail := dir
		for {
			if best, ok := x.bySuffix[tail]; !ok || len(dir) < len(best) || (len(dir) == len(best) && dir < best) {
				x.bySuffix[tail] = dir
			}
			i := strings.IndexByte(tail, '/')
			if i < 0 {
				break
			}
			tail = tail[i+1:]
		}
	}
	return x
}

// Has reports whether a directory holds a file of the scan.
func (x *DirIndex) Has(dir string) bool {
	return x.dirs[dir]
}

// ShortestEndingIn finds the directory a path names, wherever the import
// root happens to sit in the tree. The shortest wins: between `src/acme/db`
// and `vendor/other/src/acme/db`, the first is the one the code means. Two
// of the same length (django-oscar's `catalogue/reviews` and
// `dashboard/reviews` for the label `reviews`) are settled by name, so the
// same code resolves the same way on every scan; map order used to decide.
func (x *DirIndex) ShortestEndingIn(path string) string {
	if path == "" || path == "." || path == "/" {
		return ""
	}
	return x.bySuffix[path]
}
