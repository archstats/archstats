package navigation

import (
	"regexp"
	"strings"
)

// Symfony routes written in YAML: a name, then an indented block with a path,
// the methods and the controller. A block with a resource and a prefix
// imports another routing file under that prefix; the prefixes are joined
// when linking, once every routing file has been read.

type routeImport struct {
	// Resource is the imported file as written: "@SyliusAdminBundle/Resources/config/routing/ajax.yml".
	Resource string
	Prefix   string
}

var (
	yamlTopKey   = regexp.MustCompile(`^([A-Za-z_][\w.\-]*):\s*$`)
	yamlField    = regexp.MustCompile(`^\s+(_?[a-z_]+):\s*(.*?)\s*$`)
	routingFiles = regexp.MustCompile(`(?i)(^|/)(config/routes?|routing)(/|\.ya?ml$|_[\w-]*\.ya?ml$)|(^|/)routes?\.ya?ml$|(^|/)routing[\w-]*\.ya?ml$`)
)

func isRoutingFile(p string) bool { return routingFiles.MatchString(p) }

// yamlRoutes reads a routing file's routes and the files it imports.
func (s *source) yamlRoutes() ([]entry, []routeImport) {
	if s.lang != langYAML || !isRoutingFile(s.path) {
		return nil, nil
	}
	var out []entry
	var imports []routeImport
	type block struct {
		line   int
		fields map[string]string
	}
	var blocks []block
	var cur *block
	for line := 1; line <= len(s.lines); line++ {
		text := s.lineText(line)
		if m := yamlTopKey.FindStringSubmatch(text); m != nil {
			blocks = append(blocks, block{line: line, fields: map[string]string{}})
			cur = &blocks[len(blocks)-1]
			continue
		}
		if cur == nil {
			continue
		}
		if m := yamlField.FindStringSubmatch(text); m != nil {
			key := m[1]
			if _, seen := cur.fields[key]; !seen {
				cur.fields[key] = strings.Trim(m[2], `'"`)
			}
		}
	}
	for _, b := range blocks {
		if res := b.fields["resource"]; res != "" {
			imports = append(imports, routeImport{Resource: res, Prefix: b.fields["prefix"]})
			continue
		}
		p, ok := b.fields["path"]
		if !ok {
			continue
		}
		e := entry{Kind: kindHTTP, Method: "ANY", Path: joinRoute("", p), Framework: "symfony", Line: b.line, File: s.path}
		if m := strings.Trim(b.fields["methods"], "[] "); m != "" {
			e.Method = strings.ToUpper(strings.Join(strings.Fields(strings.ReplaceAll(m, ",", " ")), ","))
		}
		ctrl := b.fields["controller"]
		if ctrl == "" {
			ctrl = b.fields["_controller"]
		}
		e.Handler = symfonyController(ctrl)
		out = append(out, e)
	}
	return out, imports
}

// symfonyController is a controller as a handler: `App\Controller\X::show`
// is X.show, `X` alone is X, and a service id is kept whole, for the linker
// to look up among the service bindings.
func symfonyController(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	cls, method := c, ""
	if i := strings.Index(c, "::"); i > 0 {
		cls, method = c[:i], c[i+2:]
	} else if i := strings.LastIndex(c, ":"); i > 0 && !strings.Contains(c, `\`) {
		cls, method = c[:i], c[i+1:]
	}
	if strings.Contains(cls, `\`) {
		cls = cls[strings.LastIndexByte(cls, '\\')+1:]
	}
	if method != "" {
		return cls + "." + method
	}
	return cls
}
