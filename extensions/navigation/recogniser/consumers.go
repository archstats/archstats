package recogniser

import (
	"path"
	"strings"
)

// A Consumer is a mark that makes a member consume something: a message, an
// event, a schedule, a command. Names are the arguments that name what.
type Consumer struct {
	Kind, Framework string
	Names           []string
}

// Consume is the entry a consumer mark makes of a member.
func (c Consumer) Consume(m Mark, e Entry) Entry {
	x := e
	x.Kind, x.Framework, x.Line = c.Kind, c.Framework, m.Line
	if len(c.Names) > 0 {
		x.Path = m.Arg(c.Names...)
		if x.Path == "" && len(m.Args) > 0 {
			x.Path = strings.TrimSpace(m.Args[0])
		}
	}
	return x
}

// MainEntry is a program's main found at an offset: the language is its
// framework, the file's folder its path.
func MainEntry(s *Source, at int, member string) Entry {
	line := s.Line(at)
	return Entry{Kind: KindMain, Framework: s.Lang, Path: path.Dir(s.Path), Line: line, DeclLine: line, Member: member}
}
