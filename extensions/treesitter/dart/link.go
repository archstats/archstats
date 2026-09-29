package dart

import (
	"path"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/module"
	"github.com/archstats/archstats/core/unit"
)

// linker resolves what a Dart file imports to the files it means, once every
// file and every pubspec is known.
//
//   - `package:<name>/x/y.dart` is `lib/x/y.dart` under the pubspec called
//     <name>, when that pubspec is in the checkout; otherwise the package is
//     a dependency and resolves to nothing.
//   - A relative URI is relative to the importing file. `dart:` is the SDK.
//   - An import of a file in another directory is a component edge, as an
//     import is in every other pack.
//   - A name a file uses resolves to the library that declares it among the
//     file itself, its parts, what it imports, and what those export: a
//     Flutter app's barrel files (`export 'src/widgets.dart';`) are how most
//     of its names travel.
//
// It runs after the component linker, which never sees a Dart import, so the
// edges it adds are its own.
type linker struct {
	root string
}

func (l *linker) EditFileResults(all []*file.Results) {
	byPath := map[string]*file.Results{}
	var dartFiles []*file.Results
	var names []string
	for _, fr := range all {
		p := strings.TrimPrefix(fr.Name, "./")
		names = append(names, p)
		if strings.HasSuffix(p, ".dart") {
			byPath[p] = fr
			dartFiles = append(dartFiles, fr)
		}
	}
	if len(dartFiles) == 0 {
		return
	}
	packages := map[string]string{} // pubspec name -> its directory
	for _, m := range module.ReadFrom(l.root, names).Modules() {
		if m.Kind == "pub" {
			if _, taken := packages[m.Name]; !taken {
				packages[m.Name] = m.Dir
			}
		}
	}
	resolve := func(from, uri string) string {
		switch {
		case strings.HasPrefix(uri, "dart:"):
			return ""
		case strings.HasPrefix(uri, "package:"):
			rest := strings.TrimPrefix(uri, "package:")
			i := strings.Index(rest, "/")
			if i < 0 {
				return ""
			}
			dir, ok := packages[rest[:i]]
			if !ok {
				return ""
			}
			return path.Clean(path.Join(dir, "lib", rest[i+1:]))
		case strings.Contains(uri, ":"):
			return ""
		}
		return path.Clean(path.Join(path.Dir(strings.TrimPrefix(from, "./")), uri))
	}

	// What each library declares at top level, and what it imports, exports
	// and includes as parts, all resolved to files in the checkout.
	type library struct {
		declares                map[string]string // name -> unit id
		imports, exports, parts []string
	}
	libs := map[string]*library{}
	for _, fr := range dartFiles {
		p := strings.TrimPrefix(fr.Name, "./")
		lib := &library{declares: map[string]string{}}
		modulePath := strings.TrimSuffix(p, ".dart")
		for _, u := range fr.Units {
			u.ID = strings.Replace(u.ID, placeholder, modulePath, 1)
			u.Owner = strings.Replace(u.Owner, placeholder, modulePath, 1)
			if u.Owner == "" {
				lib.declares[u.Name] = u.ID
			}
		}
		for _, s := range fr.Snippets {
			var list *[]string
			switch s.Type {
			case captureImport:
				list = &lib.imports
			case captureExport:
				list = &lib.exports
			case capturePart:
				list = &lib.parts
			default:
				continue
			}
			if target := resolve(fr.Name, s.Value); target != "" && byPath[target] != nil {
				*list = append(*list, target)
			}
		}
		libs[p] = lib
	}
	// A part's code is its library's: the two see each other's names, and
	// the part sees what the library imports, having no imports of its own.
	libraryOf := map[string]string{}
	for p, lib := range libs {
		for _, part := range lib.parts {
			if pl := libs[part]; pl != nil {
				pl.parts = append(pl.parts, p)
				libraryOf[part] = p
			}
		}
	}
	// Everything a library makes visible to whoever imports it: itself, its
	// parts, and what it exports, followed a few levels.
	var exported func(p string, depth int, seen map[string]bool, out *[]string)
	exported = func(p string, depth int, seen map[string]bool, out *[]string) {
		if seen[p] || depth > 4 {
			return
		}
		seen[p] = true
		*out = append(*out, p)
		lib := libs[p]
		if lib == nil {
			return
		}
		for _, part := range lib.parts {
			if !seen[part] {
				seen[part] = true
				*out = append(*out, part)
			}
		}
		for _, e := range lib.exports {
			exported(e, depth+1, seen, out)
		}
	}

	for _, fr := range dartFiles {
		p := strings.TrimPrefix(fr.Name, "./")
		lib := libs[p]
		var visible []string
		seen := map[string]bool{}
		exported(p, 0, seen, &visible)
		for _, imp := range lib.imports {
			exported(imp, 0, seen, &visible)
		}
		if owner := libs[libraryOf[p]]; owner != nil {
			for _, imp := range owner.imports {
				exported(imp, 0, seen, &visible)
			}
		}
		lookup := func(name string) (string, bool) {
			for _, v := range visible {
				if id, ok := libs[v].declares[name]; ok {
					return id, true
				}
			}
			return "", false
		}
		for _, u := range fr.Units {
			var refs []unit.Ref
			for _, r := range u.Refs {
				if r.Module != placeholder {
					refs = append(refs, r)
					continue
				}
				if id, ok := lookup(r.Name); ok && id != u.ID {
					i := strings.LastIndex(id, "#")
					refs = append(refs, unit.Ref{Module: id[:i], Name: id[i+1:], Exact: true})
				}
			}
			u.Refs = refs
		}

		reached := map[string]bool{}
		kept := fr.Snippets[:0]
		var edges []*file.Snippet
		for _, s := range fr.Snippets {
			switch s.Type {
			case captureRef, captureExport, capturePart:
				continue
			case captureImport:
				target := resolve(fr.Name, s.Value)
				to := byPath[target]
				if to == nil || to.Directory == fr.Directory || reached[to.Directory] {
					continue
				}
				reached[to.Directory] = true
				edges = append(edges, &file.Snippet{
					File: s.File, Directory: fr.Directory, Component: s.Component,
					Type: file.ComponentImport, Value: to.Directory, Begin: s.Begin, End: s.End,
				})
				continue
			}
			kept = append(kept, s)
		}
		fr.Snippets = append(kept, edges...)
		fr.Stats = append(fr.Stats, file.SnippetsToStats(edges)...)
	}
}
