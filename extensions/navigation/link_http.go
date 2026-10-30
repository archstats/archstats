package navigation

import (
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
)

// Linking the HTTP picture: routes get an id, a template, their access rule
// and their contract; contracts get their operations and design findings;
// outbound calls get their unit, function, address, system and the route or
// consumer they reach; every entry point gets its trace.

type operationRow struct {
	Spec, Title, Version, Method, Path, Template, OperationID, Tag, Summary, Security, Side string
	Deprecated                                                                              bool
	Entry, Line                                                                             int
}

type outboundRow struct {
	ID                                                                     int
	Kind, Client, Method, Path, Template, Target, System, Host, Confidence string
	ResolvedVia, ConfigKey, Resilience, Unit, Function, Component, File    string
	Entry, Consumer, Line                                                  int
}

type systemRow struct {
	System, Kind, Environment, Host, Source, ConfigKey, File string
	Line                                                     int
}

type configRow struct {
	Key, Value, Environment, Format, File string
	Line                                  int
}

// configIndex is every kept configuration entry, by its normalised key.
type configIndex struct {
	byKey    map[string][]cfgEntry
	services map[string]bool
	tuning   map[string][]string // normalised key -> resilience kinds
	// ctxs are the context paths the application is served under.
	ctxs []string
}

// A routeAt is a route as reached under one context path.
type routeAt struct {
	r *entryRow
	t string
}

// routeIndex is every HTTP route under every context path, by segment count.
type routeIndex map[string][]routeAt

// match is the route a template and method name: the one whose segments
// agree most exactly, a parameter matching a literal only when nothing
// better does, and the method named before ANY.
func (idx routeIndex) match(template, method string) *entryRow {
	var best *entryRow
	bestScore := -1
	for _, ra := range idx[segmentsKey(template)] {
		if !sameTemplate(ra.t, template) || !methodsMatch(ra.r.Method, method) {
			continue
		}
		score := 0
		a, b := strings.Split(ra.t, "/"), strings.Split(template, "/")
		for i := range a {
			if a[i] == b[i] || strings.EqualFold(a[i], b[i]) {
				score += 2
			}
		}
		if ra.r.Method == method {
			score++
		}
		if score > bestScore {
			best, bestScore = ra.r, score
		}
	}
	return best
}

func (e *extension) linkHTTP(l *linker, files map[string]*facts, paths []string) {
	// Routes: ids and templates.
	for i := range e.entryRows {
		r := &e.entryRows[i]
		r.ID = i + 1
		if r.Kind == kindHTTP || r.Kind == kindPage {
			r.Template = pathTemplate(r.Path)
		}
	}

	cfg := e.indexConfig(l, files, paths)
	routes := routeIndex{}
	for i := range e.entryRows {
		r := &e.entryRows[i]
		if r.Kind != kindHTTP {
			continue
		}
		for _, c := range cfg.ctxs {
			t := withBase(c, r.Template)
			routes[segmentsKey(t)] = append(routes[segmentsKey(t)], routeAt{r, t})
		}
	}
	e.linkAuth(l, files, paths, cfg)
	consumed := e.linkContracts(l, files, paths, routes)
	e.linkOutbound(l, files, paths, cfg, routes, consumed)
	e.trace(l, files)
}

func (e *extension) indexConfig(l *linker, files map[string]*facts, paths []string) *configIndex {
	idx := &configIndex{byKey: map[string][]cfgEntry{}, services: map[string]bool{}, tuning: map[string][]string{}, ctxs: []string{"/"}}
	for _, p := range paths {
		cf := files[p].config
		if cf == nil || !own(l.results, p) {
			continue
		}
		for _, c := range cf.entries {
			k := normKey(c.Key)
			c.Format = c.Format + "|" + p
			idx.byKey[k] = append(idx.byKey[k], c)
			e.configRows = append(e.configRows, configRow{Key: c.Key, Value: c.Value, Environment: c.Env, Format: strings.SplitN(c.Format, "|", 2)[0], File: p, Line: c.Line})
		}
		for _, t := range cf.tuning {
			k := normKey(t.Key)
			idx.tuning[k] = append(idx.tuning[k], t.Kind)
		}
		for _, s := range cf.services {
			idx.services[strings.ToLower(s.Name)] = true
		}
	}
	for _, k := range []string{"server.servlet.context-path", "server.context-path", "spring.webflux.base-path", "spring.mvc.servlet.path", "spring.jersey.application-path"} {
		for _, v := range idx.values(k, 0) {
			if b := basePath(v); b != "/" && !contains(idx.ctxs, b) {
				idx.ctxs = append(idx.ctxs, b)
			}
		}
	}
	return idx
}

// values are the values a key has, per environment, references followed.
func (c *configIndex) values(key string, depth int) map[string]string {
	out := map[string]string{}
	if depth > 4 {
		return out
	}
	for _, en := range c.byKey[normKey(key)] {
		v := en.Value
		if ref, def, ok := refersTo(v); ok {
			resolved := c.values(ref, depth+1)
			if r, ok := resolved[en.Env]; ok {
				v = reference.ReplaceAllString(v, r)
			} else if r, ok := resolved[""]; ok {
				v = reference.ReplaceAllString(v, r)
			} else if def != "" {
				v = reference.ReplaceAllString(v, def)
			}
			// Other environments the reference has values for.
			for env, r := range resolved {
				if _, have := out[env]; !have && env != "" && en.Env == "" {
					out[env] = reference.ReplaceAllString(en.Value, r)
				}
			}
		}
		if _, have := out[en.Env]; !have {
			out[en.Env] = v
		}
	}
	return out
}

// fileOf is where a key is first set.
func (c *configIndex) fileOf(key string) (string, int) {
	for _, en := range c.byKey[normKey(key)] {
		parts := strings.SplitN(en.Format, "|", 2)
		if len(parts) == 2 {
			return parts[1], en.Line
		}
	}
	return "", 0
}

func (e *extension) linkAuth(l *linker, files map[string]*facts, paths []string, cfg *configIndex) {
	var rules []authRule
	// Security rules see the path below the context path but above the
	// servlet a framework is mounted on: JAX-RS's @ApplicationPath, Jersey's
	// or the dispatcher servlet's path.
	mounts := []string{"/"}
	for _, p := range paths {
		if fx := files[p]; fx.syms != nil {
			for _, a := range fx.syms.AppPaths {
				if b := basePath(a); !contains(mounts, b) {
					mounts = append(mounts, b)
				}
			}
		}
	}
	for _, k := range []string{"spring.jersey.application-path", "spring.mvc.servlet.path"} {
		for _, v := range cfg.values(k, 0) {
			if b := basePath(v); !contains(mounts, b) {
				mounts = append(mounts, b)
			}
		}
	}
	ranges := map[string][]authRange{}
	classAuth := map[string]string{}
	for _, p := range paths {
		fx := files[p]
		if fx.syms != nil {
			rules = append(rules, fx.syms.AuthRules...)
			if len(fx.syms.AuthRanges) > 0 {
				ranges[p] = fx.syms.AuthRanges
			}
		}
		for cls, a := range fx.classAuth {
			classAuth[p+"\x00"+cls] = a
		}
	}
	for i := range e.entryRows {
		r := &e.entryRows[i]
		if r.Kind != kindHTTP || r.Auth != "" {
			continue
		}
		for _, rg := range ranges[r.File] {
			if r.Line >= rg.From && r.Line <= rg.To {
				r.Auth = rg.Rule
			}
		}
		if r.Auth == "" && r.Unit != "" {
			// The handler's class, then the classes it extends.
			u := l.results.UnitByID[r.Unit]
			for depth := 0; u != nil && depth < 5 && r.Auth == ""; depth++ {
				for _, f := range u.Files {
					if a, ok := classAuth[clean(f)+"\x00"+u.Name]; ok {
						r.Auth = a
					}
				}
				var next *unit.Unit
				for _, s := range e.superRows {
					if s.Unit == u.ID && s.Supertype != "" {
						next = l.results.UnitByID[s.Supertype]
						break
					}
				}
				u = next
			}
		}
		if r.Auth == "" {
		rules:
			for _, rule := range rules {
				for _, m := range mounts {
					if antMatch(rule.Pattern, withBase(m, r.Template)) && methodsMatch(rule.Method, r.Method) && (rule.Method == "" || r.Method != "ANY") {
						r.Auth = rule.Rule
						break rules
					}
				}
			}
		}
	}
}

// linkContracts reads every contract, matches its operations to routes and
// returns the consumed operations by operationId, for finding the calls a
// generated client makes.
func (e *extension) linkContracts(l *linker, files map[string]*facts, paths []string, routes routeIndex) map[string][]operationRow {
	sides := map[string]string{}
	for _, p := range paths {
		for _, g := range files[p].gens {
			sp := specPath(p, g.Spec)
			if sp == "" {
				// A remote spec: a system this code calls, named by its host.
				if g.Side == "consumed" && strings.Contains(g.Spec, "://") {
					e.systemRows = append(e.systemRows, systemRow{System: hostOf(g.Spec), Kind: g.Kind, Host: hostOf(g.Spec), Source: "generator", File: p, Line: g.Line})
				}
				continue
			}
			sides[sp] = g.Side
		}
	}
	matched := map[int]bool{}
	apiComponents := map[string]bool{}
	consumed := map[string][]operationRow{}
	for _, p := range paths {
		s := files[p].spec
		if s == nil {
			continue
		}
		if r := roleOf(l.results, p); r == "third_party" {
			continue
		}
		side := sides[p]
		bases := []string{"/"}
		for _, sv := range s.Servers {
			if b := basePath(sv.URL); !contains(bases, b) {
				bases = append(bases, b)
			}
		}
		var rows []operationRow
		hits := 0
		for _, op := range s.Ops {
			row := operationRow{Spec: p, Title: s.Title, Version: s.APIVer, Method: op.Method, Path: op.Path, Template: pathTemplate(op.Path), OperationID: op.OperationID, Tag: op.Tag, Summary: op.Summary, Security: op.Security, Deprecated: op.Deprecated, Line: op.Line}
			if side != "consumed" {
				for _, b := range bases {
					if r := routes.match(withBase(b, row.Template), op.Method); r != nil {
						row.Entry = r.ID
						r.Documented = "spec"
						matched[r.ID] = true
						apiComponents[r.Component] = true
						hits++
						break
					}
				}
			}
			rows = append(rows, row)
		}
		switch {
		case side != "":
		case hits > 0:
			side = "provided"
		default:
			side = "unknown"
		}
		for i := range rows {
			rows[i].Side = side
			if side == "consumed" && rows[i].OperationID != "" {
				consumed[rows[i].OperationID] = append(consumed[rows[i].OperationID], rows[i])
			}
		}
		e.operationRows = append(e.operationRows, rows...)
		if side != "consumed" {
			e.findingRows = append(e.findingRows, specFindings(s)...)
		}
		if side == "consumed" {
			for _, sv := range s.Servers {
				if h := hostOf(sv.URL); h != "" && !isLocalHost(h) {
					e.systemRows = append(e.systemRows, systemRow{System: systemFromSpec(s), Kind: "http", Host: h, Source: "generator", File: p, Line: sv.Line})
				}
			}
		}
	}
	for i := range e.entryRows {
		if r := &e.entryRows[i]; r.Documented == "" && r.Framework == "fastapi" {
			r.Documented = "framework"
		}
	}
	e.findingRows = append(e.findingRows, routeFindings(e.entryRows, matched, apiComponents)...)
	sort.SliceStable(e.findingRows, func(i, j int) bool {
		a, b := e.findingRows[i], e.findingRows[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})
	return consumed
}

// segmentsKey groups templates by their segment count, the first cut of a
// template match.
func segmentsKey(t string) string { return itoa(strings.Count(t, "/")) }

func systemFromSpec(s *spec) string {
	if s.Title != "" {
		return s.Title
	}
	return strings.TrimSuffix(path.Base(s.File), path.Ext(s.File))
}

// A resolved address: the host per environment, the path, how it was found.
type resolvedAddr struct {
	hosts      map[string]string // environment -> host
	path       string
	via        string
	configKey  string
	unresolved string // the part that could not be followed, as written
}

func (e *extension) linkOutbound(l *linker, files map[string]*facts, paths []string, cfg *configIndex, routes routeIndex, consumed map[string][]operationRow) {
	// Bases by name across the codebase: beans, typed and named HttpClients,
	// Refit interfaces.
	global := map[string]clientBase{}
	prefixes := map[string]string{}
	for _, p := range paths {
		for _, b := range files[p].bases {
			if _, ok := global[b.Name]; !ok {
				global[b.Name] = b
			}
		}
		if files[p].syms != nil {
			for t, pre := range files[p].syms.Prefixes {
				prefixes[t] = pre
			}
		}
	}
	consumers := map[string][]*entryRow{}
	for i := range e.entryRows {
		r := &e.entryRows[i]
		switch r.Kind {
		case kindMessage, kindJob, kindEvent:
			consumers[strings.TrimSpace(r.Path)] = append(consumers[strings.TrimSpace(r.Path)], r)
			if r.Function != "" {
				fn := r.Function[strings.LastIndexByte(r.Function, '.')+1:]
				consumers["fn:"+fn] = append(consumers["fn:"+fn], r)
			}
		}
	}

	expand := func(addr string, local []clientBase, recv, class string) resolvedAddr {
		a := strings.TrimSpace(addr)
		// A path with no base: the base bound to the client it went through.
		if strings.HasPrefix(a, "/") || !strings.Contains(a, "://") && !strings.HasPrefix(a, "{") {
			for _, b := range local {
				if b.Name == recv && b.Addr != "" {
					a = joinAddr(b.Addr, a)
					break
				}
			}
			if !strings.Contains(a, "://") && !strings.HasPrefix(a, "{") {
				for _, name := range []string{recv, class, strings.TrimPrefix(class, "I")} {
					if b, ok := global[name]; ok && name != "" && b.Addr != "" {
						a = joinAddr(b.Addr, a)
						break
					}
				}
			}
		}
		return e.resolveAddr(a, cfg, prefixes, local, global)
	}

	nextID := 1
	for _, p := range paths {
		fx := files[p]
		if len(fx.outs) == 0 || !own(l.results, p) {
			continue
		}
		for _, c := range fx.outs {
			row := outboundRow{ID: nextID, Kind: c.Kind, Client: c.Client, Method: c.Method, File: p, Line: c.Line, Component: componentOf(l.results, p), Resilience: c.Resilience}
			nextID++
			// Where the call is.
			if c.Declared {
				u := l.unitNamed(p, c.Class)
				row.Unit = idOf(u)
				member := l.memberAt(p, c.Line)
				if u != nil && member != "" {
					row.Function = u.Name + "." + member
					if fn := l.functionNamed(u, member); fn != nil {
						row.Function = fn.Name
					}
				}
			} else {
				fn := l.functionAt(p, c.Line)
				u := l.unitOfFunction(p, fn)
				if u == nil {
					u = l.unitNamed(p, l.typeAt(p, c.Line))
				}
				if u == nil && len(l.units[p]) == 1 {
					u = l.units[p][0]
				}
				row.Unit = idOf(u)
				if fn != nil {
					row.Function = fn.Name
					for ln := fn.Begin - 1; ln <= fn.Begin+1; ln++ {
						if r, ok := fx.resMarks[ln]; ok {
							row.Resilience = mergeResilience(row.Resilience, r)
						}
					}
				}
			}
			addr := c.Addr
			if c.Declared && c.Base != "" {
				addr = joinAddr(rawToAddr(c.Base), addr)
			}
			class := c.Class
			if class == "" {
				class = l.typeAt(p, c.Line)
			}
			ra := expand(addr, fx.bases, c.Recv, class)
			// What the client's registration adds: Polly, a timeout.
			for _, name := range []string{c.Recv, class} {
				if b, ok := global[name]; ok && name != "" && b.Resilience != "" {
					row.Resilience = mergeResilience(row.Resilience, b.Resilience)
				}
			}
			row.Path = displayAddr(ra.path)
			row.ConfigKey = ra.configKey
			row.ResolvedVia = ra.via
			row.Target = displayAddr(addr)
			if c.Kind == "message" || c.Kind == "grpc" && c.Target != "" {
				row.Path = strings.Trim(displayAddr(c.Addr), "/")
				if row.Path == "" {
					row.Path = c.Target
				}
			}
			if c.Kind == "http" {
				row.Template = pathTemplate(ra.path)
			}
			row.Host = pickHost(ra.hosts)
			if c.Declared {
				row.ResolvedVia = orDefault(row.ResolvedVia, "declared")
			}
			row.System = systemName(c, ra, row.Host, cfg)
			// Resilience a client's configuration tunes: feign.client.config.payments.*.
			for k, kinds := range cfg.tuning {
				if name := normKey(row.System); len(name) >= 3 && strings.Contains(k, name) {
					for _, kind := range kinds {
						row.Resilience = mergeResilience(row.Resilience, kind)
					}
				}
			}
			// The route or consumer a call reaches inside the workspace.
			internal := row.Host == "" || isLocalHost(row.Host) || cfg.services[strings.ToLower(row.Host)]
			// A path of parameters only ("/{id}") matches any one-segment route.
			if c.Kind == "http" && internal && row.Template != "" && strings.Trim(strings.ReplaceAll(row.Template, "{}", ""), "/") != "" {
				if r := routes.match(row.Template, row.Method); r != nil {
					row.Entry = r.ID
					if row.Host == "" {
						row.System = "self"
					}
				}
			}
			if c.Kind == "message" {
				key := row.Path
				if c.Client == "celery" {
					key = "fn:" + c.Target
				}
				if list := consumers[key]; len(list) > 0 {
					row.Consumer = list[0].ID
				}
			}
			row.Confidence = confidence(c, ra, row)
			e.outboundRows = append(e.outboundRows, row)
			e.addSystem(row, ra, cfg)
		}
	}

	// Calls through a client generated from a consumed contract:
	// petApi.getPetById(id) is GET /pet/{petId}.
	if len(consumed) > 0 {
		for _, p := range paths {
			fx := files[p]
			if fx.syms == nil || !own(l.results, p) {
				continue
			}
			for _, cs := range fx.syms.Calls {
				ops, ok := consumed[cs.Name]
				if !ok || cs.Recv == "" {
					continue
				}
				t := genericless(fx.syms.Vars[lastSegment(cs.Recv)])
				if !rx(`(Api|Client|ApiClient|Service)$`).MatchString(t) {
					continue
				}
				op := ops[0]
				fn := l.functionAt(p, int(cs.Line))
				u := l.unitOfFunction(p, fn)
				row := outboundRow{ID: nextID, Kind: "http", Client: "openapi-generator", Method: op.Method, Path: op.Path, Template: op.Template, Target: op.Title + " " + op.OperationID, System: op.Title, Confidence: "declared", ResolvedVia: "generated", Unit: idOf(u), Component: componentOf(l.results, p), File: p, Line: int(cs.Line)}
				if fn != nil {
					row.Function = fn.Name
				}
				nextID++
				e.outboundRows = append(e.outboundRows, row)
			}
		}
	}

	// Systems the configuration names that no call reached: a datastore, a
	// broker, an identity provider, an HTTP service.
	for _, entries := range cfg.byKey {
		for _, en := range entries {
			kind := kindOfConfig(en.Key, en.Value)
			if kind == "" {
				continue
			}
			v := en.Value
			if ref, def, ok := refersTo(v); ok {
				vals := cfg.values(ref, 0)
				if r, ok := vals[en.Env]; ok {
					v = r
				} else if r, ok := vals[""]; ok {
					v = r
				} else {
					v = def
				}
			}
			host := hostOf(v)
			if host == "" {
				continue
			}
			file := strings.SplitN(en.Format, "|", 2)
			src := systemRow{System: systemFromConfig(en.Key, v, kind), Kind: kind, Environment: en.Env, Host: host, Source: "config", ConfigKey: en.Key, Line: en.Line}
			if len(file) == 2 {
				src.File = file[1]
			}
			e.systemRows = append(e.systemRows, src)
		}
	}
	e.systemRows = dedupeSystems(e.systemRows)
}

// memberAt is the name of the member declared on or just below a line.
func (l *linker) memberAt(f string, line int) string {
	for _, fn := range l.functions[f] {
		if fn.Begin >= line && fn.Begin <= line+3 {
			return fn.Name[strings.LastIndexByte(fn.Name, '.')+1:]
		}
	}
	return ""
}

// rawToAddr renders an annotation's address: `${payments.url}` is a
// configuration key, anything else a literal.
func rawToAddr(raw string) string {
	r := strings.TrimSpace(raw)
	if m := rx(`^\$\{([^}:]+)(?::([^}]*))?\}(.*)$`).FindStringSubmatch(r); m != nil {
		return "{config:" + m[1] + "}" + m[3]
	}
	return r
}

var marker = rx(`\{(config|env|prop|var|const):([^{}]*)\}`)

// displayAddr writes an address the way a person reads it: `${partner.url}/orders/{id}`.
func displayAddr(a string) string {
	return marker.ReplaceAllStringFunc(a, func(m string) string {
		g := marker.FindStringSubmatch(m)
		switch g[1] {
		case "config":
			return "${" + g[2] + "}"
		case "env":
			return "$" + g[2]
		case "prop":
			return strings.Replace(g[2], ":", ".", 1)
		}
		return "{" + g[2] + "}"
	})
}

// resolveAddr follows an address's leading marker to its value in each
// environment, and splits off the path.
func (e *extension) resolveAddr(a string, cfg *configIndex, prefixes map[string]string, local []clientBase, global map[string]clientBase) resolvedAddr {
	ra := resolvedAddr{hosts: map[string]string{}}
	for step := 0; step < 4; step++ {
		loc := marker.FindStringSubmatchIndex(a)
		if loc == nil || loc[0] != 0 {
			break
		}
		kind, name, rest := a[loc[2]:loc[3]], a[loc[4]:loc[5]], a[loc[1]:]
		if kind == "var" {
			// A variable a base was bound to: follow it and read again.
			if b := baseNamed(name, local, global); b != "" {
				a = joinAddr(b, rest)
				continue
			}
		}
		var vals map[string]string
		switch kind {
		case "config", "env":
			vals = cfg.values(name, 0)
			ra.configKey = orDefault(ra.configKey, name)
			ra.via = orDefault(ra.via, kind)
		case "prop":
			parts := strings.SplitN(name, ":", 2)
			if pre, ok := prefixes[parts[0]]; ok && len(parts) == 2 {
				key := pre + "." + parts[1]
				vals = cfg.values(key, 0)
				ra.configKey = orDefault(ra.configKey, key)
				ra.via = orDefault(ra.via, "config")
			}
		}
		if len(vals) == 0 {
			ra.unresolved = displayAddr(a[:loc[1]])
			ra.path = rest
			return ra
		}
		paths := map[string]string{}
		for env, v := range vals {
			full := joinAddr(v, rest)
			if h := hostOf(full); h != "" {
				ra.hosts[env] = h
			}
			paths[env] = pathOf(full)
		}
		// The path of the environment whose host is shown.
		ra.path = paths[preferredEnv(paths)]
		return ra
	}
	if strings.Contains(a, "://") {
		ra.via = orDefault(ra.via, "literal")
		ra.hosts[""] = hostOf(a)
		ra.path = pathOf(a)
		return ra
	}
	if loc := marker.FindStringIndex(a); loc != nil && loc[0] == 0 {
		ra.unresolved = displayAddr(a[:loc[1]])
		ra.path = a[loc[1]:]
		return ra
	}
	ra.path = a
	if strings.HasPrefix(a, "/") {
		ra.via = orDefault(ra.via, "literal")
	}
	return ra
}

func baseNamed(name string, local []clientBase, global map[string]clientBase) string {
	for _, b := range local {
		if b.Name == name && b.Addr != "" {
			return b.Addr
		}
	}
	if b, ok := global[name]; ok {
		return b.Addr
	}
	return ""
}

// hostOf is the host an address names: a URL's, a JDBC URL's, a connection
// string's Server, the first of a broker list.
func hostOf(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	v = strings.TrimPrefix(v, "jdbc:")
	if i := strings.Index(v, "://"); i >= 0 {
		rest := v[i+3:]
		if at := strings.LastIndexByte(strings.SplitN(rest, "/", 2)[0], '@'); at >= 0 {
			rest = rest[at+1:]
		}
		end := strings.IndexAny(rest, "/?#,;")
		if end >= 0 {
			rest = rest[:end]
		}
		if u, err := url.Parse("x://" + rest); err == nil && u.Hostname() != "" {
			return strings.ToLower(u.Hostname())
		}
		return strings.ToLower(strings.Split(rest, ":")[0])
	}
	if m := rx(`(?i)(?:^|;)\s*(?:server|data source|host|address|addr|endpoint)\s*=\s*(?:tcp:)?([^;,\s]+)`).FindStringSubmatch(v); m != nil {
		return strings.ToLower(strings.Split(m[1], ":")[0])
	}
	if hostPort.MatchString(v) {
		return strings.ToLower(strings.Split(strings.Split(v, ",")[0], ":")[0])
	}
	if rx(`^[a-z0-9][a-z0-9.-]*\.[a-z]{2,}$`).MatchString(strings.ToLower(v)) {
		return strings.ToLower(v)
	}
	return ""
}

// pathOf is an address's path after its scheme and host.
func pathOf(v string) string {
	if i := strings.Index(v, "://"); i >= 0 {
		rest := v[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			return rest[j:]
		}
		return "/"
	}
	return v
}

func isLocalHost(h string) bool {
	switch strings.ToLower(h) {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1", "host.docker.internal", "":
		return true
	}
	return false
}

// pickHost is the host to show for an address: production's, then the
// default, then acceptance, then any.
func pickHost(hosts map[string]string) string {
	return hosts[preferredEnv(hosts)]
}

// preferredEnv is the environment whose value is shown: production, the
// default, acceptance, test, development, then the first by name.
func preferredEnv(values map[string]string) string {
	for _, env := range []string{"prod", "", "acc", "test", "dev", "local"} {
		if v, ok := values[env]; ok && v != "" {
			return env
		}
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if values[k] != "" {
			return k
		}
	}
	return ""
}

var keyNoise = map[string]bool{"url": true, "uri": true, "urls": true, "base": true, "baseurl": true, "baseuri": true, "endpoint": true, "host": true, "hostname": true, "address": true, "addr": true, "api": true, "apiurl": true, "client": true, "clients": true, "service": true, "services": true, "external": true, "integration": true, "integrations": true, "rest": true, "http": true, "app": true, "application": true, "config": true, "settings": true, "connectionstrings": true, "path": true, "server": true, "servers": true, "spring": true, "cloud": true, "openfeign": true, "feign": true, "datasource": true, "data": true, "default": true, "properties": true, "props": true, "options": true, "react": true, "next": true, "public": true, "vite": true, "nuxt": true, "env": true, "remote": true, "target": true, "location": true, "root": true, "rooturi": true}

// systemName names what an outbound call reaches, best evidence first.
func systemName(c outCall, ra resolvedAddr, host string, cfg *configIndex) string {
	if c.Target != "" && c.Kind != "message" {
		return c.Target
	}
	if ra.configKey != "" {
		if n := nameFromKey(ra.configKey); n != "" {
			return n
		}
	}
	if host != "" && !isLocalHost(host) {
		return host
	}
	switch c.Kind {
	case "message":
		return c.Client
	case "http":
		if strings.HasPrefix(ra.path, "/") && ra.unresolved == "" && len(ra.hosts) == 0 {
			return "self"
		}
	}
	// An address that could not be followed names no system: the call is
	// listed under its client, never under a variable's name.
	return ""
}

// nameFromKey is the meaningful part of a configuration key:
// `clients.payments.base-url` is payments, PARTNER_KVK_URL is partner-kvk.
func nameFromKey(key string) string {
	var keep []string
	for _, part := range rx(`[.:_\-/]+`).Split(key, -1) {
		p := strings.ToLower(part)
		if p == "" || keyNoise[p] || keyNoise[normKey(p)] {
			continue
		}
		keep = append(keep, p)
	}
	if len(keep) == 0 {
		return ""
	}
	if len(keep) > 2 {
		keep = keep[len(keep)-2:]
	}
	return strings.Join(keep, "-")
}

func confidence(c outCall, ra resolvedAddr, row outboundRow) string {
	switch {
	case c.Declared:
		return "declared"
	case row.Host != "" || row.Entry > 0 || row.Consumer > 0 || row.System == "self":
		return "resolved"
	case ra.configKey != "" || c.Target != "" || c.Kind == "message" && row.Path != "" && !strings.HasPrefix(row.Path, "{"):
		return "partial"
	}
	return "unresolved"
}

var (
	kindByScheme = map[string]string{"jdbc": "sql", "postgres": "sql", "postgresql": "sql", "mysql": "sql", "mariadb": "sql", "sqlserver": "sql", "oracle": "sql", "mongodb": "nosql", "mongodb+srv": "nosql", "redis": "cache", "rediss": "cache", "amqp": "message", "amqps": "message", "kafka": "message", "nats": "message", "ldap": "ldap", "ldaps": "ldap", "smtp": "mail", "smtps": "mail", "sftp": "file_transfer", "ftp": "file_transfer", "ftps": "file_transfer", "http": "http", "https": "http", "elasticsearch": "search", "s3": "storage"}
	kindByKey    = []struct {
		re   *regexp.Regexp
		kind string
	}{
		{rx(`(?i)datasource|database|jdbc|connectionstrings|[^a-z]db[^a-z]|^db|r2dbc|sql`), "sql"},
		{rx(`(?i)mongo|cosmos|dynamo|couch|cassandra`), "nosql"},
		{rx(`(?i)redis|memcache|cache`), "cache"},
		{rx(`(?i)kafka|rabbit|amqp|broker|bootstrap|servicebus|nats|activemq|artemis|jms|pubsub|sqs|sns`), "message"},
		{rx(`(?i)elastic|opensearch|solr`), "search"},
		{rx(`(?i)mail|smtp`), "mail"},
		{rx(`(?i)ldap`), "ldap"},
		{rx(`(?i)issuer|jwk|oauth2|oidc|openid|authority|keycloak|saml|idp|digid|eherkenning`), "identity"},
		{rx(`(?i)wsdl|soap|serviceModel`), "soap"},
		{rx(`(?i)s3|blob|bucket|storage|minio`), "storage"},
	}
)

// kindOfConfig is what kind of system a configuration entry points at, or ""
// when it is not an address worth a system.
func kindOfConfig(key, value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	scheme := ""
	if i := strings.Index(v, "://"); i > 0 {
		scheme = strings.TrimPrefix(v[:i], "jdbc:")
		if strings.HasPrefix(v, "jdbc:") {
			scheme = "jdbc"
		}
	}
	for _, k := range kindByKey {
		if k.re.MatchString(key) {
			if k.kind == "sql" && (scheme == "http" || scheme == "https") {
				break
			}
			return k.kind
		}
	}
	if kind, ok := kindByScheme[scheme]; ok {
		if kind == "http" && !placeKey.MatchString(key) {
			return ""
		}
		return kind
	}
	if rx(`(?i)^(server|data source|host)\s*=`).MatchString(v) {
		return "sql"
	}
	return ""
}

func systemFromConfig(key, value, kind string) string {
	if n := nameFromKey(key); n != "" && kind == "http" || kind == "identity" || kind == "soap" {
		if n != "" {
			return n
		}
	}
	h := hostOf(value)
	switch kind {
	case "sql", "nosql":
		db := ""
		if p := pathOf(strings.TrimPrefix(value, "jdbc:")); strings.HasPrefix(p, "/") {
			db = strings.Trim(strings.SplitN(p, "?", 2)[0], "/")
		}
		if m := rx(`(?i)(?:database|initial catalog)\s*=\s*([^;]+)`).FindStringSubmatch(value); m != nil {
			db = m[1]
		}
		if db != "" {
			return kind + ":" + db
		}
	}
	if n := nameFromKey(key); n != "" {
		return n
	}
	return h
}

func (e *extension) addSystem(row outboundRow, ra resolvedAddr, cfg *configIndex) {
	if row.System == "" || row.System == "self" {
		return
	}
	if len(ra.hosts) == 0 {
		e.systemRows = append(e.systemRows, systemRow{System: row.System, Kind: row.Kind, Source: "call", ConfigKey: row.ConfigKey, File: row.File, Line: row.Line})
		return
	}
	for env, h := range ra.hosts {
		sr := systemRow{System: row.System, Kind: row.Kind, Environment: env, Host: h, Source: "call", ConfigKey: row.ConfigKey, File: row.File, Line: row.Line}
		if row.ConfigKey != "" {
			if f, ln := cfg.fileOf(row.ConfigKey); f != "" {
				sr.Source, sr.File, sr.Line = "config", f, ln
			}
		}
		e.systemRows = append(e.systemRows, sr)
	}
}

func dedupeSystems(rows []systemRow) []systemRow {
	seen := map[string]bool{}
	var out []systemRow
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.System != b.System {
			return a.System < b.System
		}
		if a.Environment != b.Environment {
			return a.Environment < b.Environment
		}
		if a.Source != b.Source {
			return a.Source > b.Source
		}
		return a.File < b.File
	})
	for _, r := range rows {
		key := strings.Join([]string{r.System, r.Kind, r.Environment, r.Host}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r)
	}
	return out
}

// The trace: from each entry point's handler, every call it makes into the
// codebase, followed through declared types, imports and implementations.

type callRow struct {
	FromUnit, FromFunction, ToUnit, ToFunction, Via, File string
	Line                                                  int
}

type reachRow struct {
	Entry, CallSites, Resolved, Unknown, Units, Functions, Tables, Outbound, Depth int
}

type targetRow struct {
	Entry  int
	Kind   string
	Target string
	Depth  int
}

type fnNode struct {
	file string
	fn   *file.Function
	unit *unit.Unit
	name string // the function's name, or Unit.member for a member with no body
}

func (n fnNode) key() string {
	if n.fn != nil {
		return n.file + "#" + itoa(n.fn.Begin)
	}
	return idOf(n.unit) + "#" + n.name
}

type traceEdge struct {
	to             fnNode
	via            string
	file           string
	line           int
	library, known bool
}

const (
	maxTraceDepth = 8
	maxTraceNodes = 400
)

func (e *extension) trace(l *linker, files map[string]*facts) {
	implementors := map[string][]*unit.Unit{}
	for _, s := range e.superRows {
		if s.Supertype != "" {
			if u := l.results.UnitByID[s.Unit]; u != nil {
				implementors[s.Supertype] = append(implementors[s.Supertype], u)
			}
		}
	}
	for _, b := range e.bindingRows {
		if b.InterfaceUnit != "" && b.ImplementationUnit != "" && b.InterfaceUnit != b.ImplementationUnit {
			if u := l.results.UnitByID[b.ImplementationUnit]; u != nil && !containsUnit(implementors[b.InterfaceUnit], u) {
				implementors[b.InterfaceUnit] = append(implementors[b.InterfaceUnit], u)
			}
		}
	}
	supers := map[string][]*unit.Unit{}
	for _, s := range e.superRows {
		if t := l.results.UnitByID[s.Supertype]; t != nil {
			supers[s.Unit] = append(supers[s.Unit], t)
		}
	}
	// What each function touches: tables and outbound calls.
	tables := map[string][]string{}
	for _, a := range e.accessRows {
		k := a.Unit + "\x00" + a.Function
		t := a.Entity
		if t == "" {
			t = a.Target
		}
		if !contains(tables[k], t) {
			tables[k] = append(tables[k], t)
		}
	}
	outs := map[string][]int{}
	for _, o := range e.outboundRows {
		k := o.Unit + "\x00" + o.Function
		outs[k] = append(outs[k], o.ID)
	}

	memo := map[string][]traceEdge{}
	edgesSeen := map[string]bool{}
	t := &tracer{l: l, files: files, implementors: implementors, supers: supers}

	for _, r := range e.entryRows {
		switch r.Kind {
		case kindHTTP, kindMessage, kindEvent, kindSchedule, kindJob:
		default:
			continue
		}
		starts := t.startNodes(r)
		if len(starts) == 0 {
			continue
		}
		reach := reachRow{Entry: r.ID}
		units := map[string]bool{}
		depthOf := map[string]int{}
		queue := append([]fnNode(nil), starts...)
		for _, s := range starts {
			units[idOf(s.unit)] = true
			depthOf[s.key()] = 0
		}
		var targets []targetRow
		addTargets := func(n fnNode, depth int) {
			// What the function touches, and what its unit touches as a
			// whole: a Spring Data repository's entity, a client interface.
			for _, k := range []string{idOf(n.unit) + "\x00" + n.name, idOf(n.unit) + "\x00"} {
				for _, tb := range tables[k] {
					targets = append(targets, targetRow{Entry: r.ID, Kind: "table", Target: tb, Depth: depth})
				}
				for _, id := range outs[k] {
					targets = append(targets, targetRow{Entry: r.ID, Kind: "outbound", Target: itoa(id), Depth: depth})
				}
			}
		}
		for _, s := range starts {
			addTargets(s, 0)
		}
		for len(queue) > 0 && len(depthOf) < maxTraceNodes {
			n := queue[0]
			queue = queue[1:]
			d := depthOf[n.key()]
			if d >= maxTraceDepth || n.fn == nil {
				continue
			}
			edges, ok := memo[n.key()]
			if !ok {
				edges = t.edges(n)
				memo[n.key()] = edges
			}
			for _, ed := range edges {
				switch {
				case ed.library:
					continue
				case !ed.known:
					reach.Unknown++
					continue
				case ed.to.key() == n.key():
					continue
				}
				reach.Resolved++
				ek := n.key() + ">" + ed.to.key()
				if !edgesSeen[ek] {
					edgesSeen[ek] = true
					e.callRows = append(e.callRows, callRow{FromUnit: idOf(n.unit), FromFunction: n.name, ToUnit: idOf(ed.to.unit), ToFunction: ed.to.name, Via: ed.via, File: ed.file, Line: ed.line})
				}
				if _, seen := depthOf[ed.to.key()]; seen {
					continue
				}
				depthOf[ed.to.key()] = d + 1
				units[idOf(ed.to.unit)] = true
				if d+1 > reach.Depth {
					reach.Depth = d + 1
				}
				addTargets(ed.to, d+1)
				queue = append(queue, ed.to)
			}
		}
		reach.CallSites = reach.Resolved + reach.Unknown
		reach.Units = len(units)
		reach.Functions = len(depthOf)
		seenT := map[string]bool{}
		for _, tg := range targets {
			k := tg.Kind + "\x00" + tg.Target
			if seenT[k] {
				continue
			}
			seenT[k] = true
			if tg.Kind == "table" {
				reach.Tables++
			} else {
				reach.Outbound++
			}
			e.targetRows = append(e.targetRows, tg)
		}
		e.reachRows = append(e.reachRows, reach)
	}
}

func containsUnit(list []*unit.Unit, u *unit.Unit) bool {
	for _, x := range list {
		if x == u {
			return true
		}
	}
	return false
}

type tracer struct {
	l            *linker
	files        map[string]*facts
	implementors map[string][]*unit.Unit
	supers       map[string][]*unit.Unit
}

// startNodes are the functions a trace begins at: the handler; for a class
// named as the handler (a Django class-based view, an invokable or Symfony
// resource controller) its methods for the route's method; for an inline
// handler the function that starts on the route's line.
func (t *tracer) startNodes(r entryRow) []fnNode {
	if n := t.startNode(r); n != nil {
		return []fnNode{*n}
	}
	u := t.l.results.UnitByID[r.Unit]
	if u == nil || u.Kind != unit.KindType {
		return nil
	}
	var out []fnNode
	for _, name := range handlerMethods(r.Method) {
		if fn, f := t.fnOf(u, name); fn != nil {
			out = append(out, fnNode{file: f, fn: fn, unit: u, name: fn.Name})
		}
	}
	return out
}

// handlerMethods are the methods a class answers a request with.
func handlerMethods(method string) []string {
	common := []string{"__invoke", "__call__", "invoke", "handle", "dispatch"}
	if method == "" || method == "ANY" || strings.Contains(method, ",") {
		return append([]string{"get", "post", "put", "patch", "delete", "Get", "Post", "Put", "Patch", "Delete", "indexAction"}, common...)
	}
	m := strings.ToLower(method)
	return append([]string{m, strings.ToUpper(m[:1]) + m[1:]}, common...)
}

// startNode is the function a trace begins at: the handler, or for an
// inline handler the function that starts on the route's line.
func (t *tracer) startNode(r entryRow) *fnNode {
	u := t.l.results.UnitByID[r.Unit]
	if r.Function != "" {
		for _, fn := range t.l.functions[r.File] {
			if fn.Name == r.Function {
				return &fnNode{file: r.File, fn: fn, unit: orUnit(u, t.l.unitOfFunction(r.File, fn)), name: fn.Name}
			}
		}
		if u != nil {
			if fn := t.l.functionNamed(u, r.Function[strings.LastIndexByte(r.Function, '.')+1:]); fn != nil {
				f := r.File
				for _, uf := range u.Files {
					for _, x := range t.l.functions[clean(uf)] {
						if x == fn {
							f = clean(uf)
						}
					}
				}
				return &fnNode{file: f, fn: fn, unit: u, name: fn.Name}
			}
		}
	}
	// An inline handler: the function that begins on the route's line or
	// the next two.
	var best *file.Function
	for _, fn := range t.l.functions[r.File] {
		if fn.Begin >= r.Line && fn.Begin <= r.Line+2 && (best == nil || fn.Begin < best.Begin || fn.Begin == best.Begin && fn.End > best.End) {
			best = fn
		}
	}
	if best != nil {
		return &fnNode{file: r.File, fn: best, unit: orUnit(u, t.l.unitOfFunction(r.File, best)), name: best.Name}
	}
	return nil
}

func orUnit(a, b *unit.Unit) *unit.Unit {
	if a != nil {
		return a
	}
	return b
}

// edges are the calls a function makes, each resolved to the function it
// reaches, marked as a library call, or left unknown.
func (t *tracer) edges(n fnNode) []traceEdge {
	fx := t.files[n.file]
	if fx == nil || fx.syms == nil {
		return nil
	}
	sy := fx.syms
	calls := sy.Calls
	lo := sort.Search(len(calls), func(i int) bool { return int(calls[i].Line) >= n.fn.Begin })
	own := n.fn.Name[strings.LastIndexByte(n.fn.Name, '.')+1:]
	var out []traceEdge
	for i := lo; i < len(calls) && int(calls[i].Line) <= n.fn.End; i++ {
		c := calls[i]
		if c.Name == own && int(c.Line) == n.fn.Begin {
			continue
		}
		for _, ed := range t.resolve(n, c, sy) {
			ed.file, ed.line = n.file, int(c.Line)
			out = append(out, ed)
		}
	}
	return out
}

func (t *tracer) resolve(n fnNode, c callSite, sy *symbols) []traceEdge {
	l := t.l
	library := []traceEdge{{library: true}}
	unknown := []traceEdge{{}}
	switch {
	case c.Recv == "":
		if n.unit != nil && n.unit.Kind == unit.KindType {
			if es := t.member(n.unit, c.Name, "self"); es != nil {
				return es
			}
		}
		if u := t.connected(n, c.Name, unit.KindFunction); u != nil {
			return t.member(u, c.Name, "import")
		}
		if u := l.unitNamed(n.file, c.Name); u != nil && u.Kind == unit.KindFunction {
			return t.member(u, c.Name, "function")
		}
		if u := l.resolve(c.Name, n.file); u != nil && u.Kind == unit.KindFunction {
			return t.member(u, c.Name, "function")
		}
		return library
	case c.Recv == "this":
		if n.unit != nil {
			if es := t.member(n.unit, c.Name, "self"); es != nil {
				return es
			}
		}
		return library
	}
	last := lastSegment(c.Recv)
	if typ, ok := sy.Vars[last]; ok {
		if u := t.resolveType(n, typ); u != nil {
			if es := t.member(u, c.Name, "field"); es != nil {
				return es
			}
			return []traceEdge{{to: fnNode{unit: u, name: u.Name + "." + c.Name}, via: "field", known: true}}
		}
		return library
	}
	if last != "" && last[0] >= 'A' && last[0] <= 'Z' {
		if u := t.resolveType(n, last); u != nil {
			if es := t.member(u, c.Name, "static"); es != nil {
				return es
			}
			return []traceEdge{{to: fnNode{unit: u, name: u.Name + "." + c.Name}, via: "static", known: true}}
		}
		return library
	}
	// A module or package: orders.Place(), services.place_order().
	if u := t.connected(n, c.Name, unit.KindFunction); u != nil {
		return t.member(u, c.Name, "import")
	}
	if strings.Contains(strings.Join(l.importsOf(n.file), " "), last) {
		return library
	}
	return unknown
}

// member is the edges a call of a member on a unit makes: the member
// itself, inherited, or in each implementation of an interface.
func (t *tracer) member(u *unit.Unit, name, via string) []traceEdge {
	if u.Kind == unit.KindFunction {
		if fn, f := t.fnOf(u, u.Name); fn != nil && u.Name == name {
			return []traceEdge{{to: fnNode{file: f, fn: fn, unit: u, name: fn.Name}, via: via, known: true}}
		}
		if u.Name == name {
			return []traceEdge{{to: fnNode{unit: u, name: u.Name}, via: via, known: true}}
		}
		return nil
	}
	if fn, f := t.fnOf(u, name); fn != nil {
		out := []traceEdge{{to: fnNode{file: f, fn: fn, unit: u, name: fn.Name}, via: via, known: true}}
		return append(out, t.implementations(u, name)...)
	}
	if impls := t.implementations(u, name); len(impls) > 0 {
		return impls
	}
	for _, s := range t.supers[u.ID] {
		if fn, f := t.fnOf(s, name); fn != nil {
			return []traceEdge{{to: fnNode{file: f, fn: fn, unit: s, name: fn.Name}, via: via, known: true}}
		}
	}
	return nil
}

func (t *tracer) implementations(u *unit.Unit, name string) []traceEdge {
	var out []traceEdge
	for i, impl := range t.implementors[u.ID] {
		if i >= 4 {
			break
		}
		if fn, f := t.fnOf(impl, name); fn != nil {
			out = append(out, traceEdge{to: fnNode{file: f, fn: fn, unit: impl, name: fn.Name}, via: "implementation", known: true})
		}
	}
	return out
}

// fnOf is a unit's member function by name and the file it is in.
func (t *tracer) fnOf(u *unit.Unit, name string) (*file.Function, string) {
	for _, f := range u.Files {
		cf := clean(f)
		for _, fn := range t.l.functions[cf] {
			last := fn.Name[strings.LastIndexByte(fn.Name, '.')+1:]
			if last != name {
				continue
			}
			if u.Kind != unit.KindType || strings.Contains(fn.Name, u.Name+".") || fn.Name == name && len(t.l.units[cf]) == 1 {
				return fn, cf
			}
		}
	}
	return nil, ""
}

// resolveType is the unit a declared type names, preferring one the
// calling unit's imports reach.
func (t *tracer) resolveType(n fnNode, typ string) *unit.Unit {
	g := strings.TrimLeft(genericless(strings.TrimSpace(typ)), "*&?")
	g = strings.TrimSuffix(strings.TrimSuffix(g, "?"), "[]")
	simple := g[strings.LastIndexAny(g, `.\`)+1:]
	if simple == "" || primitive[strings.ToLower(simple)] {
		return nil
	}
	if u := t.connected(n, simple, unit.KindType); u != nil {
		return u
	}
	if u := t.l.resolve(g, n.file); u != nil && u.Kind == unit.KindType {
		return u
	}
	return nil
}

// connected is a unit of a name and kind the calling unit, or any unit in
// its file, has an edge to.
func (t *tracer) connected(n fnNode, name, kind string) *unit.Unit {
	from := t.l.connectionsFrom()
	var sources []string
	if n.unit != nil {
		sources = append(sources, n.unit.ID)
	}
	for _, u := range t.l.units[n.file] {
		if n.unit == nil || u.ID != n.unit.ID {
			sources = append(sources, u.ID)
		}
	}
	for _, s := range sources {
		for _, to := range from[s] {
			if u := t.l.results.UnitByID[to]; u != nil && u.Name == name && u.Kind == kind {
				return u
			}
		}
	}
	return nil
}
