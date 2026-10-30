package recogniser

import (
	"regexp"
	"strings"
)

// ReadNested reads a nested PHP array (`'url' => env('X')`) or Python dict
// (`'HOST': os.environ.get('X')`) into dotted keys, by following the
// brackets that open under a key.
func ReadNested(text, arrow, root string, add func(k, v string, line int, env string)) {
	keyed := regexp.MustCompile(`['"]([\w.-]+)['"]\s*` + regexp.QuoteMeta(arrow) + `\s*`)
	var stack []string
	depth := 0
	pushedAt := map[int]bool{}
	line := 1
	i := 0
	for i < len(text) {
		c := text[i]
		switch c {
		case '\n':
			line++
			i++
			continue
		case '#':
			if arrow == ":" {
				end := strings.IndexByte(text[i:], '\n')
				if end < 0 {
					return
				}
				i += end
				continue
			}
		case '[', '{', '(':
			depth++
			i++
			continue
		case ']', '}', ')':
			if pushedAt[depth] {
				pushedAt[depth] = false
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
			depth--
			i++
			continue
		case '\'', '"':
			loc := keyed.FindStringSubmatchIndex(text[i:])
			if loc != nil && loc[0] == 0 {
				key := text[i+loc[2] : i+loc[3]]
				j := i + loc[1]
				if j < len(text) && (text[j] == '[' || text[j] == '{' || strings.HasPrefix(text[j:], "array(")) {
					if strings.HasPrefix(text[j:], "array(") {
						j += 5
					}
					depth++
					pushedAt[depth] = true
					stack = append(stack, key)
					i = j + 1
					continue
				}
				end := j
				for end < len(text) && text[end] != ',' && text[end] != '\n' {
					if text[end] == '(' {
						_, e := Balanced(text, end)
						end = e
						continue
					}
					end++
				}
				full := strings.Join(append(append([]string{}, stack...), key), ".")
				if root != "" {
					full = root + "." + full
				}
				add(full, strings.TrimSpace(text[j:end]), line, "")
				line += strings.Count(text[i:end], "\n")
				i = end
				continue
			}
			i = SkipString(text, i)
			continue
		}
		i++
	}
}
