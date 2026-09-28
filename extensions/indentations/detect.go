package indentations

import "bytes"

// tabWidth is how many spaces stand for a tab where nothing says otherwise.
const tabWidth = 4

// detectWidth reads a file's indentation step from the file itself: the
// increase most often taken from one line's indentation to the next. A file
// indented mostly with tabs answers tabWidth; its tabs count one level each
// whatever the width. It reports false when the file never steps in.
//
// Only increases count. A dedent can close several levels at once, and a
// one-space step is a block comment's " * " continuation, not a level.
func detectWidth(content []byte) (int, bool) {
	var tabbed, spaced int
	steps := map[int]int{}
	previous := 0
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		spaces := 0
		for spaces < len(line) && line[spaces] == ' ' {
			spaces++
		}
		if line[spaces] == '\t' {
			tabbed++
			continue
		}
		if spaces > 0 {
			spaced++
		}
		if step := spaces - previous; step >= 2 && step <= 8 {
			steps[step]++
		}
		previous = spaces
	}
	if tabbed > spaced {
		return tabWidth, true
	}
	best, bestCount := 0, 0
	for step, count := range steps {
		if count > bestCount || (count == bestCount && step < best) {
			best, bestCount = step, count
		}
	}
	return best, bestCount > 0
}
