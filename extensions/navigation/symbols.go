package navigation

import (
	"regexp"
	"sort"
	"strings"

	r "github.com/archstats/archstats/extensions/navigation/recogniser"
)

// What a route reaches is read one call at a time: `orders.place(cart)` in a
// handler is a call into whatever type `orders` was declared as. A file's
// symbols are what it takes to follow such a call without a compiler: the
// declared type of every field, property, parameter and constructed
// variable, the string constants, which fields hold configuration, and every
// call site with the line it is on. Scope is the file, not the block: two
// variables of one name in one file with different types are rare enough to
// accept the first. The language's recognisers read its declarations; the
// shapes every language shares are read here.

var (
	// x = new Foo(, x := NewFoo(, x = Foo(, self.x = Foo(, x = &Foo{.
	constructed = regexp.MustCompile(`(?:\b(?:self|this)\s*(?:\.|->)\s*|\$this->|\b)([a-z_$][\w$]*)\s*(?::=|=)\s*(?:await\s+)?(?:new\s+|&)?((?:[a-z_]\w*\.)?(?:New)?[A-Z][\w]*)\s*[({]`)
	// self.x = x: a field that holds what a typed parameter was.
	assignedFrom = regexp.MustCompile(`(?:\b(?:self|this)\s*\.\s*|\$this->)(\w+)\s*=\s*\$?(\w+)\s*;?\s*$`)
	callPattern  = regexp.MustCompile(`([A-Za-z_$][\w$]*(?:\s*(?:\.|->|\?\.|::)\s*[A-Za-z_$][\w$]*)*)\s*(?:\.|->|\?\.|::)\s*([A-Za-z_$][\w$]*)\s*(?:<[^<>()]*>\s*)?\(`)
	barePattern  = regexp.MustCompile(`(?:^|[^\w$.>:@\\])([A-Za-z_][\w]*)\s*\(`)
)

func symbolsOf(s *source) *symbols {
	sy := r.NewSymbols()
	recs := s.Language().Recognisers()
	code := s.Code
	for _, rec := range recs {
		if rec.Declared != nil {
			rec.Declared(s, sy)
		}
	}
	for _, m := range constructed.FindAllStringSubmatch(code, -1) {
		typ := m[2]
		for _, rec := range recs {
			if rec.Constructed != nil {
				typ = rec.Constructed(typ)
			}
		}
		sy.SetVar(m[1], typ)
	}
	for _, rec := range recs {
		if rec.Modules != nil {
			rec.Modules(s, sy)
		}
	}
	// self.repo = repo: the field takes the parameter's type.
	for line := 1; line <= len(s.Lines); line++ {
		if m := assignedFrom.FindStringSubmatch(s.LineText(line)); m != nil && m[1] != m[2] {
			if t, ok := sy.Vars[m[2]]; ok {
				sy.SetVar(m[1], t)
			}
		}
	}

	for _, rec := range recs {
		if rec.Constants != nil {
			rec.Constants(s, sy)
		}
	}
	for _, rec := range recs {
		if rec.ConfigKeys != nil {
			rec.ConfigKeys(s, sy)
		}
	}

	// Call sites.
	for _, m := range callPattern.FindAllStringSubmatchIndex(code, -1) {
		recv := strings.Join(strings.Fields(code[m[2]:m[3]]), "")
		sy.Calls = append(sy.Calls, callSite{Recv: sy.In(normalRecv(recv)), Name: sy.In(code[m[4]:m[5]]), Line: int32(s.Line(m[0])), At: int32(m[0]), Open: int32(m[1] - 1)})
	}
	for _, m := range barePattern.FindAllStringSubmatchIndex(code, -1) {
		name := code[m[2]:m[3]]
		if notCalls[name] || notCalls[strings.ToLower(name)] {
			continue
		}
		sy.Calls = append(sy.Calls, callSite{Name: sy.In(name), Line: int32(s.Line(m[2])), At: int32(m[2]), Open: int32(m[1] - 1)})
	}
	sort.SliceStable(sy.Calls, func(i, j int) bool { return sy.Calls[i].Line < sy.Calls[j].Line })

	for _, rec := range recs {
		if rec.Security != nil {
			rec.Security(s, sy)
		}
	}
	return sy
}

// classAuth is what each class in a file says about access to every route
// on it.
func classAuth(s *source) map[string]string {
	var out map[string]string
	for _, rec := range s.Language().Recognisers() {
		if rec.ClassAuth == nil {
			continue
		}
		for k, v := range rec.ClassAuth(s) {
			if out == nil {
				out = map[string]string{}
			}
			out[k] = v
		}
	}
	return out
}

var primitive = map[string]bool{"string": true, "int": true, "long": true, "boolean": true, "bool": true, "double": true, "float": true, "byte": true, "short": true, "char": true, "object": true, "integer": true, "number": true, "any": true, "void": true, "str": true, "dict": true, "list": true, "optional": true, "map": true, "set": true, "date": true, "datetime": true, "localdate": true, "localdatetime": true, "instant": true, "uuid": true, "bigdecimal": true, "decimal": true, "guid": true, "unit": true, "nothing": true, "array": true, "record": true, "promise": true, "observable": true, "task": true, "error": true, "exception": true, "context": true, "request": true, "response": true, "httpservletrequest": true, "httpservletresponse": true, "model": true, "pageable": true, "principal": true, "authentication": true, "logger": true, "log": true, "ilogger": true, "cancellationtoken": true}
