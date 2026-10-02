package navigation

import (
	"regexp"
	"strings"
)

// Where an implementation is bound to what it implements. The interface lives
// in one place, the implementation in another, and a third wires them, so the
// import graph shows the first two as unrelated and the third as touching
// both for no visible reason. A binding is read where a container is told it
// outright; a type a container finds by scanning is added when linking, from
// its annotations and what it implements.

type binding struct {
	// Interface is what is asked for: a type's name, or a bean or service id
	// when a container names it by string.
	Interface      string
	Implementation string
	Mechanism      string
	File           string
	Line           int
	// DeclLine is a @Bean method's declaration, for reading what it returns.
	DeclLine int
}

var (
	guiceBind       = regexp.MustCompile(`\bbind\s*\(\s*([A-Z][\w.]*)(?:\.class|::class)\s*\)\s*\.\s*to\s*\(\s*([A-Z][\w.]*)(?:\.class|::class)`)
	dotnetAdd       = regexp.MustCompile(`\.(?:Add|TryAdd)(?:Scoped|Transient|Singleton)\s*<\s*([A-Z][\w.]*(?:<[^<>]*>)?)\s*,\s*([A-Z][\w.]*(?:<[^<>]*>)?)\s*>`)
	dotnetTypeof    = regexp.MustCompile(`\.(?:Add|TryAdd)(?:Scoped|Transient|Singleton)\s*\(\s*typeof\s*\(\s*([A-Z][\w.<>,]*)\s*\)\s*,\s*typeof\s*\(\s*([A-Z][\w.<>,]*)\s*\)`)
	autofac         = regexp.MustCompile(`RegisterType\s*<\s*([A-Z][\w.]*)\s*>\s*\(\s*\)[^;]*?\.As\s*<\s*([A-Z][\w.]*)\s*>`)
	laravelBind     = regexp.MustCompile(`->(?:bind|singleton|scoped|bindIf|singletonIf)\s*\(\s*\\?([A-Z][\w\\]*)::class\s*,\s*\\?([A-Z][\w\\]*)::class`)
	angularProv     = regexp.MustCompile(`\bprovide\s*:\s*([A-Z]\w*)\s*,\s*use(?:Class|Existing)\s*:\s*([A-Z]\w*)`)
	koinSingle      = regexp.MustCompile(`\b(?:single|factory|scoped)\s*<\s*([A-Z]\w*)\s*>\s*\{\s*([A-Z]\w*)\s*\(`)
	hiltBinds       = regexp.MustCompile(`@Binds\b[^;{}]*?\bfun\s+\w+\s*\(\s*\w+\s*:\s*([A-Z]\w*)\s*\)\s*:\s*([A-Z]\w*)`)
	springBean      = regexp.MustCompile(`<bean\b[^>]*>`)
	xmlAttr         = regexp.MustCompile(`\b(id|name|class)\s*=\s*"([^"]*)"`)
	symfonySvc      = regexp.MustCompile(`<service\b[^>]*>`)
	symfonyAlias    = regexp.MustCompile(`<service\b[^>]*\balias\s*=\s*"([^"]+)"[^>]*\bid\s*=\s*"([^"]+)"|<service\b[^>]*\bid\s*=\s*"([^"]+)"[^>]*\balias\s*=\s*"([^"]+)"`)
	symfonySet      = regexp.MustCompile(`->set\(\s*['"]([^'"]+)['"]\s*,\s*\\?([A-Z][\w\\]*)::class`)
	symfonyPHPAlias = regexp.MustCompile(`->alias\(\s*\\?([A-Z][\w\\]*)::class\s*,\s*['"]([^'"]+)['"]`)
	newObject       = regexp.MustCompile(`\bnew\s+([A-Z][\w.]*)\s*[(<{]`)
	kotlinCtor      = regexp.MustCompile(`=\s*([A-Z]\w*)\s*\(`)
	beanReturn      = regexp.MustCompile(`(?:^|\s)([A-Z][\w.]*(?:<[^()]*>)?)\s+[a-z_]\w*\s*\(`)
	kotlinReturn    = regexp.MustCompile(`\)\s*:\s*([A-Z][\w.]*)`)
)

func (s *source) bindings() []binding {
	var out []binding
	add := func(iface, impl, mechanism string, offset int) {
		out = append(out, binding{Interface: strings.TrimSpace(iface), Implementation: strings.TrimSpace(impl), Mechanism: mechanism, File: s.path, Line: s.line(offset)})
	}
	pairs := func(re *regexp.Regexp, mechanism string, implFirst bool) {
		for _, m := range re.FindAllStringSubmatchIndex(s.code, -1) {
			a, b := s.code[m[2]:m[3]], s.code[m[4]:m[5]]
			if implFirst {
				a, b = b, a
			}
			add(a, b, mechanism, m[0])
		}
	}
	switch s.lang {
	case langJava, langKotlin, langScala:
		pairs(guiceBind, "guice", false)
		if s.lang == langKotlin {
			pairs(koinSingle, "koin", false)
			pairs(hiltBinds, "hilt", true)
		}
		for _, d := range s.decls() {
			if d.Class || d.has("Bean", "Provides") == nil {
				continue
			}
			mechanism := "bean"
			if d.has("Provides") != nil {
				mechanism = "dagger"
			}
			returns := ""
			if m := beanReturn.FindStringSubmatch(" " + d.Text); m != nil && s.lang != langKotlin {
				returns = m[1]
			} else if m := kotlinReturn.FindStringSubmatch(d.Text); m != nil {
				returns = m[1]
			}
			if i := strings.IndexByte(returns, '<'); i > 0 {
				returns = returns[:i]
			}
			if returns == "" {
				continue
			}
			out = append(out, binding{Interface: returns, Implementation: beanImplementation(s.body(d.Line)), Mechanism: mechanism, File: s.path, Line: d.Marks[0].Line, DeclLine: d.Line})
		}
	case langCSharp:
		pairs(dotnetAdd, "dotnet_di", false)
		pairs(dotnetTypeof, "dotnet_di", false)
		pairs(autofac, "autofac", true)
	case langPHP:
		pairs(laravelBind, "laravel", false)
		// Symfony's PHP service config: ->set('id', Class::class) and
		// ->alias(Interface::class, 'id').
		pairs(symfonySet, "symfony_php", false)
		pairs(symfonyPHPAlias, "symfony_alias", false)
	case langJS:
		pairs(angularProv, "angular", false)
	case langXML:
		if strings.Contains(s.code, "springframework.org/schema/beans") || strings.Contains(s.code, "<beans") {
			for _, loc := range springBean.FindAllStringIndex(s.code, -1) {
				attrs := map[string]string{}
				for _, a := range xmlAttr.FindAllStringSubmatch(s.code[loc[0]:loc[1]], -1) {
					attrs[a[1]] = a[2]
				}
				if attrs["class"] == "" {
					continue
				}
				id := attrs["id"]
				if names := strings.Fields(strings.ReplaceAll(attrs["name"], ",", " ")); id == "" && len(names) > 0 {
					id = names[0]
				}
				add(id, attrs["class"], "spring_xml", loc[0])
			}
		}
		if strings.Contains(s.code, "symfony.com/schema/dic/services") {
			for _, loc := range symfonySvc.FindAllStringIndex(s.code, -1) {
				attrs := map[string]string{}
				for _, a := range xmlAttr.FindAllStringSubmatch(s.code[loc[0]:loc[1]], -1) {
					attrs[a[1]] = a[2]
				}
				if attrs["id"] != "" && attrs["class"] != "" {
					add(attrs["id"], attrs["class"], "symfony_xml", loc[0])
				}
			}
			for _, m := range symfonyAlias.FindAllStringSubmatchIndex(s.code, -1) {
				if m[2] >= 0 {
					add(s.code[m[4]:m[5]], s.code[m[2]:m[3]], "symfony_alias", m[0])
				} else {
					add(s.code[m[6]:m[7]], s.code[m[8]:m[9]], "symfony_alias", m[0])
				}
			}
		}
	}
	return out
}

// body is the code of the member declared on a line: its braces, or for an
// expression body (Kotlin's `fun x() = X()`) the rest of the line.
func (s *source) body(line int) string {
	start := s.lines[line-1]
	rest := s.code[start:]
	brace := strings.IndexByte(rest, '{')
	eq := strings.Index(rest[:min(len(rest), max(brace, 0)+1)], "=")
	if brace < 0 || eq >= 0 && eq < brace && s.lang == langKotlin {
		return s.lineText(line)
	}
	inner, _ := balanced(s.code, start+brace)
	return inner
}

// beanImplementation is what a @Bean method constructs: the first `new X(`
// in its body, or Kotlin's `= X(`.
func beanImplementation(body string) string {
	if m := newObject.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	if m := kotlinCtor.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}

// Annotations under which a container finds a type by scanning, and binds it
// to what it implements.
var scanned = map[string]bool{
	"Component": true, "Service": true, "Repository": true, "Controller": true, "RestController": true,
	"Named": true, "Singleton": true, "ApplicationScoped": true, "RequestScoped": true, "Stateless": true,
	"Injectable": true, "AutoConfiguration": true,
}
