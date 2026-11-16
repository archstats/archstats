package ecmascript

// jsoncState is where the scanner stands in a JSON-with-comments document.
type jsoncState int

const (
	jsoncCode         jsoncState = iota // between tokens
	jsoncString                         // inside "..."
	jsoncEscape                         // after a backslash inside a string
	jsoncLineComment                    // after //, until the newline
	jsoncBlockComment                   // after /*, until */
)

// toJSON turns JSON with comments and trailing commas, as tsconfig and
// jsconfig allow, into JSON encoding/json accepts. Comment and string
// markers inside strings are text: "@/*" and "src/**/*" stay as they are.
func toJSON(src []byte) []byte {
	out := make([]byte, 0, len(src))
	comma := -1 // index in out of a comma that may turn out to be trailing
	state := jsoncCode
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch state {
		case jsoncString:
			out = append(out, c)
			switch c {
			case '\\':
				state = jsoncEscape
			case '"':
				state = jsoncCode
			}
		case jsoncEscape:
			out = append(out, c)
			state = jsoncString
		case jsoncLineComment:
			if c == '\n' {
				out = append(out, c)
				state = jsoncCode
			}
		case jsoncBlockComment:
			if c == '*' && i+1 < len(src) && src[i+1] == '/' {
				i++
				state = jsoncCode
			}
		case jsoncCode:
			switch {
			case c == '/' && i+1 < len(src) && src[i+1] == '/':
				i++
				state = jsoncLineComment
			case c == '/' && i+1 < len(src) && src[i+1] == '*':
				i++
				state = jsoncBlockComment
			case c == ' ' || c == '\t' || c == '\n' || c == '\r':
				out = append(out, c)
			case (c == '}' || c == ']') && comma >= 0:
				out[comma] = ' '
				comma = -1
				out = append(out, c)
			default:
				comma = -1
				if c == ',' {
					comma = len(out)
				}
				if c == '"' {
					state = jsoncString
				}
				out = append(out, c)
			}
		}
	}
	return out
}
