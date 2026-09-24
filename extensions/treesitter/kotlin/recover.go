package kotlin

import (
	"regexp"

	"github.com/archstats/archstats/core/file"
)

// recoverDeclarations finds the types the parser lost.
//
// The Kotlin grammar misreads some valid code, and the error does not stay
// where it starts: in Exposed's ColumnType.kt one misread member turns the
// rest of the file into a single broken expression, and 42 of its 44 types
// -- ULongColumnType, BinaryColumnType and the rest -- were never units, so
// nothing anywhere could depend on them. Across Exposed 94 of 2,363 type
// declarations were lost that way.
//
// A declaration is plain to read even where the tree is not: a keyword and a
// name at the start of a line, then a body in braces. Each one the parser
// did not produce is returned as a name and a span, in the same shape the
// queries capture, so it is named, nested and given its references exactly
// like the rest.
func recoverDeclarations(content []byte, captured []*file.Snippet) (names, spans []*file.Snippet) {
	have := map[int]bool{}
	for _, s := range captured {
		if s.Type == captureType || s.Type == captureObject {
			have[s.Begin.Offset] = true
		}
	}
	masked := maskCommentsAndStrings(content)
	for _, m := range declarationLine.FindAllSubmatchIndex(masked, -1) {
		nameBegin, nameEnd := m[4], m[5]
		if have[nameBegin] {
			continue
		}
		end := declarationEnd(masked, nameEnd)
		names = append(names, &file.Snippet{
			Type:  captureType,
			Value: string(content[nameBegin:nameEnd]),
			Begin: &file.Position{Offset: nameBegin},
			End:   &file.Position{Offset: nameEnd},
		})
		spans = append(spans, &file.Snippet{
			Type:  captureTypeSpan,
			Begin: &file.Position{Offset: m[0]},
			End:   &file.Position{Offset: end},
		})
	}
	return names, spans
}

var declarationLine = regexp.MustCompile(`(?m)^[ \t]*(?:@\w+(?:\([^()\n]*\))?[ \t]+)*` +
	`(?:(?:public|private|internal|protected|data|sealed|abstract|open|enum|annotation|inner|value|inline|expect|actual|final|fun)[ \t]+)*` +
	`(class|object|interface)[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)

// declarationEnd is where a declaration starting just before from ends: the
// brace closing its body, or the end of its header line when it has none.
func declarationEnd(src []byte, from int) int {
	depth := 0
	for i := from; i < len(src); i++ {
		switch src[i] {
		case '(', '<', '[':
			depth++
		case ')', '>', ']':
			if depth > 0 {
				depth--
			}
		case '{':
			if depth == 0 {
				return matchingBrace(src, i)
			}
		case '\n':
			// A header continues on the next line only inside brackets or
			// when that line carries on the header (`: Base`, `where`).
			if depth == 0 && !continuesHeader(src, i+1) {
				return i
			}
		}
	}
	return len(src)
}

func continuesHeader(src []byte, at int) bool {
	for at < len(src) && (src[at] == ' ' || src[at] == '\t') {
		at++
	}
	if at >= len(src) {
		return false
	}
	switch src[at] {
	case ':', ',', '{', '(':
		return true
	}
	return len(src)-at >= 5 && string(src[at:at+5]) == "where"
}

func matchingBrace(src []byte, open int) int {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(src)
}

// maskCommentsAndStrings blanks comments and string contents, keeping every
// offset and newline where it was, so a brace or a keyword in either is not
// read as code.
func maskCommentsAndStrings(src []byte) []byte {
	out := append([]byte(nil), src...)
	blank := func(from, to int) {
		for i := from; i < to && i < len(out); i++ {
			if out[i] != '\n' {
				out[i] = ' '
			}
		}
	}
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '/':
			j := i
			for j < len(src) && src[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			j := i + 2
			for j+1 < len(src) && !(src[j] == '*' && src[j+1] == '/') {
				j++
			}
			blank(i, j+2)
			i = j + 1
		case i+2 < len(src) && string(src[i:i+3]) == `"""`:
			j := i + 3
			for j+2 < len(src) && string(src[j:j+3]) != `"""` {
				j++
			}
			blank(i+3, j)
			i = j + 2
		case src[i] == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' && src[j] != '\n' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			blank(i+1, j)
			i = j
		case src[i] == '\'':
			j := i + 1
			for j < len(src) && src[j] != '\'' && src[j] != '\n' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			blank(i+1, j)
			i = j
		}
	}
	return out
}
