package navigation

import (
	"regexp"
	"strings"
)

// A typeDecl is a type declared in a file, where, and its declaration line:
// what places a type unit on a line and gives it a signature, and what tells
// a member which type it sits in.
type typeDecl struct {
	Name      string
	Line      int
	Signature string
}

var (
	leadingMarks = regexp.MustCompile(`^\s*(?:(?:@[\w.]+(?:\([^()]*\))?|\[[\w.]+(?:\([^()]*\))?\]|#\[[^\]]*\])\s*)+`)
	goType       = regexp.MustCompile(`^type\s+([A-Za-z_]\w*)(?:\[[^\]]*\])?\s+(struct|interface|func|map|\[\]|chan|[A-Za-z_*])`)
	tsAlias      = regexp.MustCompile(`^\s*(?:export\s+)?(?:declare\s+)?type\s+([A-Za-z_$][\w$]*)\s*(?:<[^=]*>)?\s*=`)
)

func (s *source) typeDecls() []typeDecl {
	if s.typesDone {
		return s.types
	}
	s.typesDone = true
	for line := 1; line <= len(s.lines); line++ {
		text := s.lineText(line)
		if strings.TrimSpace(text) == "" {
			continue
		}
		var name string
		switch s.lang {
		case langPython:
			if m := pyClass.FindStringSubmatch(text); m != nil {
				name = m[1]
			}
		case langGo:
			if m := goType.FindStringSubmatch(text); m != nil {
				name = m[1]
			}
		case langUnreadable, langMarkdown, langXML, langYAML, langPrisma:
			return nil
		default:
			t := leadingMarks.ReplaceAllString(text, "")
			if m := classDecl.FindStringSubmatch(t); m != nil {
				name = m[2]
			} else if s.lang == langJS {
				if m := tsAlias.FindStringSubmatch(t); m != nil {
					name = m[1]
				}
			}
		}
		if name == "" {
			continue
		}
		s.types = append(s.types, typeDecl{Name: name, Line: line, Signature: typeSignature(leadingMarks.ReplaceAllString(text, ""))})
	}
	return s.types
}

// typeSignature is a type's declaration line up to its body.
func typeSignature(text string) string {
	t := strings.TrimSpace(text)
	if i := strings.IndexByte(t, '{'); i >= 0 {
		t = t[:i]
	}
	t = strings.Join(strings.Fields(t), " ")
	if r := []rune(t); len(r) > 200 {
		t = string(r[:199]) + "…"
	}
	return strings.TrimSpace(t)
}
