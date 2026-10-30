package python

import (
	"path/filepath"
	"strings"

	"github.com/archstats/archstats/core/module"
)

// ---------------------------------------------------------------------------
// django: Python
//
// Django names no module in any manifest a parser can rely on -- the list
// lives in a settings file as `INSTALLED_APPS`, and django-oscar builds its
// own at `src/oscar/__init__.py`. What is reliable is `apps.py`: every app
// has one, django-oscar has 29, and the directory holding it is the app.
//
// The role in Django is in the filename, not in any annotation: `models.py`,
// `views.py`, `forms.py`, `admin.py`. That is read elsewhere, as a marker;
// here `apps.py` only marks the boundary.
// ---------------------------------------------------------------------------

type djangoReader struct{}

func (djangoReader) Kind() string         { return "django" }
func (djangoReader) Claims(b string) bool { return b == "apps.py" }

func (djangoReader) Read(root, path string) []*module.Module {
	dir := module.RelDir(root, path)
	if dir == "" {
		return nil
	}
	// The app is named the way Python would import it: the dotted path from
	// the nearest directory that is not itself a package. `src/oscar/order`
	// is the app `oscar.order`, because `src` holds no __init__.py.
	segments := strings.Split(dir, "/")
	start := 0
	for i := range segments {
		probe := filepath.Join(root, filepath.Join(segments[:i+1]...), "__init__.py")
		if _, ok := module.ReadFile(probe); ok {
			start = i
			break
		}
	}
	name := strings.Join(segments[start:], ".")
	if name == "" {
		name = segments[len(segments)-1]
	}
	return []*module.Module{{
		Name:     name,
		Dir:      dir,
		Manifest: module.RelFile(root, path),
	}}
}

// ManifestReaders reads Django's apps.py.
func ManifestReaders() []module.Reader { return []module.Reader{djangoReader{}} }
