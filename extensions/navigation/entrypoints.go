package navigation

import (
	"regexp"
	"sort"
	"strings"
)

// Where the outside world gets into the code: an HTTP route, a page, a message
// or event consumed, a schedule, a command, a program's main. Architects think
// in use cases, and a use case starts at one of these. Every one is read by
// the file's language's recognisers from how a framework spells it; none is
// inferred from a name.

func entries(s *source) []entry {
	var out []entry
	recs := s.Language().Recognisers()
	decls := s.Decls()
	classes := classContexts(s, decls)
	for _, d := range decls {
		if d.Class {
			for _, r := range recs {
				if r.ClassEntries != nil {
					out = append(out, r.ClassEntries(s, d)...)
				}
			}
			continue
		}
		class := enclosing(classes, d.Line)
		if class.Client {
			// A client's members are calls it makes, not routes.
			continue
		}
		e := memberEntry(s, d, class)
		for _, r := range recs {
			if r.MemberEntries != nil {
				out = append(out, r.MemberEntries(s, d, class, e)...)
			}
		}
	}
	for _, r := range recs {
		if r.CallEntries != nil {
			out = append(out, r.CallEntries(s)...)
		}
	}
	for _, r := range recs {
		if r.MainEntries != nil {
			out = append(out, r.MainEntries(s)...)
		}
	}
	for i := range out {
		out[i].File = s.Path
	}
	for _, r := range recs {
		if r.FileEntries != nil {
			out = append(out, r.FileEntries(s.Path)...)
		}
	}
	return out
}

// memberMethods are the methods each member's marks restrict it to, by name.
func memberMethods(s *source) map[string]string {
	var out map[string]string
	for _, rec := range s.Language().Recognisers() {
		if rec.MemberMethods == nil {
			continue
		}
		for _, d := range s.Decls() {
			if d.Class {
				continue
			}
			if m := rec.MemberMethods(d); m != "" {
				if out == nil {
					out = map[string]string{}
				}
				out[d.Name] = m
			}
		}
	}
	return out
}

// classContexts are the types in a file with what their marks say about the
// members below them, in order of declaration.
func classContexts(s *source, decls []decl) []classContext {
	var out []classContext
	marked := map[int]decl{}
	for _, d := range decls {
		if d.Class {
			marked[d.Line] = d
		}
	}
	recs := s.Language().Recognisers()
	for _, t := range s.TypeDecls() {
		c := classContext{Name: t.Name, Line: t.Line}
		if d, ok := marked[t.Line]; ok {
			for _, m := range d.Marks {
				for _, r := range recs {
					if r.ClassMark != nil {
						r.ClassMark(m, &c)
					}
				}
				if isAuthMark(s, m.Name) && c.Auth == "" {
					c.Auth = m.Text()
				}
				if isDocMark(s, m.Name) {
					c.Documented = true
				}
			}
		}
		out = append(out, c)
	}
	return out
}

func enclosing(classes []classContext, line int) classContext {
	var found classContext
	for _, c := range classes {
		if c.Line <= line {
			found = c
		}
	}
	return found
}

// memberEntry is what every entry an annotated member is shares: its lines,
// its class, and what its marks and its class's say about access and its
// contract.
func memberEntry(s *source, d decl, class classContext) entry {
	e := entry{Member: d.Name, Class: class.Name, Line: d.Marks[0].Line, DeclLine: d.Line, Auth: class.Auth}
	if class.Documented {
		e.Documented = "annotations"
	}
	for _, m := range d.Marks {
		if m.Line < e.Line {
			e.Line = m.Line
		}
		if isAuthMark(s, m.Name) {
			// The member's own mark overrides its class's.
			if e.Auth == class.Auth {
				e.Auth = ""
			}
			e.Auth = strings.TrimPrefix(e.Auth+", "+m.Text(), ", ")
		}
		if isDocMark(s, m.Name) {
			e.Documented = "annotations"
		}
	}
	return e
}

// authLike is a mark whose name says it guards access, for the ones a
// codebase defines itself: [AuthorizeAdmin], [CheckPermission(...)],
// @RequiresTenantAccess, @UseGuards-style decorators.
var authLike = regexp.MustCompile(`(?i)authori[sz]|permission|^checkaccess|^requires?(auth|role|login|scope|user|tenant|access)|guard|^roles?$|^secured$|^isgranted$|anonymous`)

// isAuthMark is whether a mark says who may call a route: one the
// language's recognisers name, or one whose name says so.
func isAuthMark(s *source, name string) bool {
	for _, r := range s.Language().Recognisers() {
		if contains(r.AuthMarks, name) {
			return true
		}
	}
	return authLike.MatchString(name) && !strings.Contains(strings.ToLower(name), "antiforgery")
}

// isDocMark is whether a mark describes a route for its contract.
func isDocMark(s *source, name string) bool {
	for _, r := range s.Language().Recognisers() {
		if contains(r.DocMarks, name) {
			return true
		}
	}
	return false
}

func sortEntries(list []entryRow) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
}
