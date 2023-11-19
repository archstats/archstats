package file

import (
	"fmt"
	"github.com/samber/lo"
)

// Position in a file. Only the offset is required.
type Position struct {
	// The offset in the file
	Offset int
	// The line number in the file. This value is not always available, and is calculated from the offset.
	Line int
	// The character position in the line. This value is not always available, and is calculated from the offset.
	CharInLine int
}

func (p *Position) String() string {
	return fmt.Sprintf("%d:%d (%d)", p.Line, p.CharInLine, p.Offset)
}

func AddLineNumberAndCharInLineToSnippets(theFile []byte, snippets []*Snippet) {
	allPositions := lo.FlatMap(snippets, func(snippet *Snippet, idx int) []*Position {
		return []*Position{snippet.Begin, snippet.End}
	})
	AddLineNumberAndCharInLineToPositions(theFile, allPositions)
}

func AddLineNumberAndCharInLineToPositions(theFile []byte, positions []*Position) {
	lineNumber, charPosition := 1, 1
	for i, b := range theFile {
		if i == len(theFile)-1 {
			// Reached the end of the byte slice, set remaining positions to end of file
			for _, pos := range positions {
				if pos.Line == 0 {
					// Set Line and CharInLine to -1 for positions that haven't been calculated
					pos.Line = lineNumber
					pos.CharInLine = charPosition
				}
			}
			break
		}
		for _, pos := range positions {
			if pos.Line == 0 && i == pos.Offset {
				// Found an offset from the list
				pos.Line = lineNumber
				pos.CharInLine = charPosition
			}
		}
		charPosition++
		if b == '\n' {
			lineNumber++
			charPosition = 1 // Reset character position at the start of a new line
		}
	}
}
