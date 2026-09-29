package deployables

import (
	stdpath "path"
	"regexp"
	"sort"
	"strings"
)

// An ownedEntry is a configuration entry and the deployables that read it.
type ownedEntry struct {
	owners []string
	e      configEntry
	source string // env, config_file, helm_values
}

// A target is what a name in configuration can resolve to.
type target struct {
	deployables []string
	external    string // a workload running an image not built here
}

func (b *builder) readConfigOwners() {
	b.owned = nil
	// A ConfigMap loaded by several workloads hands each of them every
	// address in it; bank-of-anthos gives all its services one list of all
	// the others. That is weaker evidence than a service's own setting, and
	// is labelled so.
	cmUsers := map[string]int{}
	for _, w := range b.workloads {
		seen := map[string]bool{}
		for _, ref := range w.EnvFrom {
			name := strings.Split(ref, "#")[0]
			if !seen[name] {
				seen[name] = true
				cmUsers[name]++
			}
		}
	}
	// Environment variables of whatever runs a deployable.
	for _, w := range b.workloads {
		owners := b.runs[w.Key]
		if len(owners) == 0 {
			continue
		}
		for _, e := range w.Env {
			b.owned = append(b.owned, ownedEntry{owners: owners, e: e, source: "env"})
		}
		for _, ref := range w.EnvFrom {
			parts := strings.Split(ref, "#")
			source := "env"
			if cmUsers[parts[0]] > 1 && len(parts) == 1 {
				source = "shared_config"
			}
			for _, e := range b.configMaps[parts[0]] {
				if len(parts) == 3 {
					if e.Key != parts[1] {
						continue
					}
					e.Key = parts[2]
				}
				b.owned = append(b.owned, ownedEntry{owners: owners, e: e, source: source})
			}
		}
		for _, ef := range w.EnvFiles {
			for _, kv := range parseDotenv(b.in.Files[ef]) {
				b.owned = append(b.owned, ownedEntry{owners: owners, e: configEntry{Key: kv.Key, Value: kv.Value, File: ef, Line: kv.Line}, source: "env"})
			}
		}
	}
	// Configuration files inside what a deployable holds.
	composeDirs := map[string]bool{}
	for _, c := range b.composes {
		composeDirs[dirOf(c)] = true
	}
	for _, f := range b.configFiles {
		// The .env beside a compose file is what compose interpolates the
		// file with, not the environment of whatever image copies the folder.
		if strings.HasPrefix(stdpath.Base(f), ".env") && composeDirs[dirOf(f)] {
			continue
		}
		owners := b.ownersOfFile(f)
		if len(owners) == 0 {
			continue
		}
		entries := b.configEntries(f)
		if name := applicationName(entries); name != "" {
			for _, o := range owners {
				b.appNames[normalizeName(name)] = appendUnique(b.appNames[normalizeName(name)], o)
			}
		}
		for _, e := range entries {
			b.owned = append(b.owned, ownedEntry{owners: owners, e: e, source: "config_file"})
		}
	}
	// Helm values describe what the chart runs.
	for _, v := range b.values {
		owners := b.valuesOwners(v)
		for _, fv := range v.Flat {
			b.owned = append(b.owned, ownedEntry{owners: owners, e: configEntry{Key: fv.Key, Value: fv.Value, File: v.File, Line: fv.Line}, source: "helm_values"})
		}
		for _, e := range listEnvEntries(v.Flat, v.File) {
			b.owned = append(b.owned, ownedEntry{owners: owners, e: e, source: "helm_values"})
		}
	}
}

func (b *builder) configEntries(f string) []configEntry {
	base := strings.ToLower(stdpath.Base(f))
	content := b.in.Files[f]
	switch {
	case strings.HasSuffix(base, ".properties"):
		return readProperties(f, content)
	case strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml"):
		entries, _ := readConfigYAML(f, content)
		return entries
	case strings.HasSuffix(base, ".json"):
		return readAppsettings(f, content)
	default:
		var out []configEntry
		for _, kv := range parseDotenv(content) {
			out = append(out, configEntry{Key: kv.Key, Value: kv.Value, File: f, Line: kv.Line})
		}
		return out
	}
}

// ownersOfFile is every deployable whose contents hold the file.
func (b *builder) ownersOfFile(f string) []string {
	var out []string
	for _, c := range b.m.Contents {
		if covers(c, f) {
			out = appendUnique(out, c.Deployable)
		}
	}
	sort.Strings(out)
	return out
}

// valuesOwners is who a values file configures: the deployables its images
// join to, those of the chart beside it, or -- when the repository builds
// exactly one thing -- that one. The client's helm-release/prod.yaml names
// no image at all; its repository has one service.
func (b *builder) valuesOwners(v *valuesFile) []string {
	if ids := b.valuesOwner[v.File]; len(ids) > 0 {
		return ids
	}
	for _, c := range b.charts {
		if c.Dir == v.Dir {
			return b.chartDeps[c.Dir]
		}
	}
	if only := b.onlyDeployableIn(b.in.RepoOf(v.File)); only != "" {
		return []string{only}
	}
	return nil
}

func (b *builder) onlyDeployableIn(repo string) string {
	found := ""
	for _, d := range b.m.Deployables {
		if d.Repository == repo {
			if found != "" {
				return ""
			}
			found = d.ID
		}
	}
	return found
}

func (b *builder) addresses() map[string][]target {
	addr := map[string][]target{}
	add := func(name string, t target) {
		name = normalizeName(name)
		if name == "" {
			return
		}
		for _, have := range addr[name] {
			if strings.Join(have.deployables, ",") == strings.Join(t.deployables, ",") && have.external == t.external {
				return
			}
		}
		addr[name] = append(addr[name], t)
	}
	workloadTarget := func(w *workload) target {
		if ids := b.runs[w.Key]; len(ids) > 0 {
			return target{deployables: ids}
		}
		return target{external: w.Name}
	}
	for _, d := range b.m.Deployables {
		add(d.Name, target{deployables: []string{d.ID}})
		for _, a := range d.aliases {
			add(a, target{deployables: []string{d.ID}})
		}
		for _, n := range d.names {
			add(n.Repo, target{deployables: []string{d.ID}})
		}
	}
	for _, w := range b.workloads {
		t := workloadTarget(w)
		add(w.Name, t)
		for _, a := range w.Aliases {
			add(a, t)
		}
	}
	for _, s := range b.services {
		var ids []string
		external := ""
		for _, w := range b.workloads {
			if w.Kind == "compose_service" || len(s.Selector) == 0 {
				continue
			}
			if s.Namespace != "" && w.Namespace != "" && s.Namespace != w.Namespace {
				continue
			}
			match := true
			for k, v := range s.Selector {
				if w.Labels[k] != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			if runs := b.runs[w.Key]; len(runs) > 0 {
				for _, id := range runs {
					ids = appendUnique(ids, id)
				}
			} else if external == "" {
				external = w.Name
			}
		}
		if len(ids) > 0 {
			sort.Strings(ids)
			add(s.Name, target{deployables: ids})
		} else if external != "" {
			add(s.Name, target{external: s.Name})
		}
	}
	for name, ids := range b.appNames {
		add(name, target{deployables: ids})
	}
	for _, c := range b.charts {
		if ids := b.chartDeps[c.Dir]; len(ids) > 0 {
			add(c.Name, target{deployables: ids})
		}
	}
	return addr
}

var adoConnection = regexp.MustCompile(`(?i)(?:^|;)\s*(?:host|server|data source|address)\s*=\s*([^;,]+)(?:,\d+)?.*?(?:;|^)\s*(?:database|initial catalog)\s*=\s*([^;]+)`)

func (b *builder) links() {
	addr := b.addresses()
	dsOwners := map[dsKey][]ownedEvidence{}
	topicOwners := map[string][]ownedEvidence{}
	seen := map[string]bool{}
	link := func(l *Link) {
		if l.From == l.To && l.ToKind == "deployable" {
			return
		}
		k := l.From + "|" + l.To + "|" + l.Kind + "|" + l.Via
		if seen[k] {
			return
		}
		seen[k] = true
		b.m.Links = append(b.m.Links, l)
	}
	resolveHost := func(owner string, e endpoint, entry configEntry, kind, mode, resolution string) {
		// The same service is often reachable under one name from several
		// files: built in compose.yaml, run from its published image in
		// compose.minimal.yaml. What this workspace builds wins.
		var ids []string
		externals := map[string]bool{}
		targets := addr[e.Host]
		if len(targets) == 0 && e.Internal {
			// qp-webform-test is qp-webform in its test environment.
			if base := stripEnvironment(e.Host); base != e.Host {
				targets = addr[base]
			}
		}
		for _, t := range targets {
			for _, id := range t.deployables {
				ids = appendUnique(ids, id)
			}
			if t.external != "" {
				externals[t.external] = true
			}
		}
		sort.Strings(ids)
		switch {
		case len(ids) == 1:
			link(&Link{From: owner, To: ids[0], ToKind: "deployable", Kind: kind, Mode: mode, Via: e.Host, File: entry.File, Line: entry.Line, Resolution: resolution})
		case len(ids) > 1:
			b.unresolved(owner, e.Host+" (could be "+strings.Join(ids, ", ")+")", entry.File, entry.Line, "ambiguous")
		case len(externals) > 0:
			link(&Link{From: owner, To: sortedKeys(externals)[0], ToKind: "external", Kind: kind, Mode: mode, Via: e.Host, File: entry.File, Line: entry.Line, Resolution: "name"})
		case !e.Internal:
			link(&Link{From: owner, To: e.Host, ToKind: "external", Kind: kind, Mode: mode, Via: e.Host, File: entry.File, Line: entry.Line, Resolution: "declared"})
		default:
			b.unresolved(owner, e.Host, entry.File, entry.Line, "not_found")
		}
	}
	for _, oe := range b.owned {
		eps := classify(oe.e.Key, oe.e.Value)
		if m := adoConnection.FindStringSubmatch(oe.e.Value); m != nil && len(eps) == 0 {
			if host, internal := normalizeHost(m[1]); host != "" {
				eps = append(eps, endpoint{Kind: "datastore", Vendor: "sql", Host: host, Database: strings.TrimSpace(m[2]), Internal: internal})
			}
		}
		for _, owner := range oe.owners {
			for _, e := range eps {
				switch e.Kind {
				case "http":
					resolution := "name"
					if oe.source == "shared_config" {
						resolution = "shared_config"
					}
					resolveHost(owner, e, oe.e, "calls", "sync", resolution)
				case "broker":
					ts := addr[e.Host]
					to, toKind := e.Host, "external"
					if len(ts) == 1 && len(ts[0].deployables) == 1 {
						to, toKind = ts[0].deployables[0], "deployable"
					} else if len(ts) == 1 && ts[0].external != "" {
						to = ts[0].external
					}
					link(&Link{From: owner, To: to, ToKind: toKind, Kind: "messages", Mode: "async", Via: e.Vendor, File: oe.e.File, Line: oe.e.Line, Resolution: "name"})
				case "datastore":
					// A database image this workspace builds is a deployable
					// like any other; the image's own settings naming itself
					// are not a use of it.
					var built []string
					for _, t := range addr[e.Host] {
						for _, id := range t.deployables {
							built = appendUnique(built, id)
						}
					}
					if len(built) == 1 {
						if built[0] == owner {
							continue
						}
						via := e.Vendor
						if e.Database != "" {
							via += "/" + e.Database
						}
						link(&Link{From: owner, To: built[0], ToKind: "deployable", Kind: "uses_datastore", Via: via, File: oe.e.File, Line: oe.e.Line, Resolution: "name"})
					} else {
						to := e.Host
						if e.Database != "" {
							to += "/" + e.Database
						}
						link(&Link{From: owner, To: to, ToKind: "external", Kind: "uses_datastore", Via: e.Vendor, File: oe.e.File, Line: oe.e.Line, Resolution: "declared"})
					}
					if e.Database != "" {
						k := dsKey{e.Vendor, e.Host, strings.ToLower(e.Database)}
						dsOwners[k] = append(dsOwners[k], ownedEvidence{owner, oe.e.File, oe.e.Line})
					}
				case "topic":
					topicOwners[e.Topic] = append(topicOwners[e.Topic], ownedEvidence{owner, oe.e.File, oe.e.Line})
				}
			}
		}
	}
	// compose depends_on
	for _, w := range b.workloads {
		owners := b.runs[w.Key]
		for _, dep := range w.DependsOn {
			for _, other := range b.workloads {
				if other.File != w.File || other.Name != dep.Key {
					continue
				}
				for _, owner := range owners {
					if ids := b.runs[other.Key]; len(ids) > 0 {
						for _, to := range ids {
							link(&Link{From: owner, To: to, ToKind: "deployable", Kind: "depends_on", Via: dep.Key, File: w.File, Line: dep.Line, Resolution: "declared"})
						}
					} else {
						link(&Link{From: owner, To: other.Name, ToKind: "external", Kind: "depends_on", Via: dep.Key, File: w.File, Line: dep.Line, Resolution: "declared"})
					}
				}
			}
		}
	}
	b.aspireLinks(link, func(vendor, host, db, owner, file string, line int) {
		k := dsKey{vendor, host, db}
		dsOwners[k] = append(dsOwners[k], ownedEvidence{owner, file, line})
	})
	// Two deployables naming the same database share it.
	for _, k := range sortedDSKeys(dsOwners) {
		b.pairs(dsOwners[k], func(a, c ownedEvidence) {
			link(&Link{From: a.owner, To: c.owner, ToKind: "deployable", Kind: "shares_datastore", Via: k.host + "/" + k.db, File: a.file, Line: a.line, Resolution: "name"})
		})
	}
	// Two deployables naming the same topic or queue talk through it.
	for _, t := range sortedKeys(topicOwners) {
		b.pairs(topicOwners[t], func(a, c ownedEvidence) {
			link(&Link{From: a.owner, To: c.owner, ToKind: "deployable", Kind: "messages", Mode: "async", Via: t, File: a.file, Line: a.line, Resolution: "name"})
		})
	}
	// An internal module inside more than one deployable couples them: a
	// change to qp-common rebuilds 16 services. One row per deployable that
	// carries it, not one per pair, which for a module in every service is
	// the square of the service count.
	modOwners := map[string][]ownedEvidence{}
	for _, c := range b.m.Contents {
		if c.Module == "" || c.Path == "." {
			continue
		}
		modOwners[c.Module] = append(modOwners[c.Module], ownedEvidence{c.Deployable, c.File, c.Line})
	}
	for _, mod := range sortedKeys(modOwners) {
		first := map[string]ownedEvidence{}
		for _, e := range modOwners[mod] {
			if have, ok := first[e.owner]; !ok || e.file < have.file || e.file == have.file && e.line < have.line {
				first[e.owner] = e
			}
		}
		if len(first) < 2 {
			continue
		}
		for _, owner := range sortedKeys(first) {
			e := first[owner]
			link(&Link{From: owner, To: mod, ToKind: "module", Kind: "shares_module", Via: itoa(len(first)) + " deployables", File: e.file, Line: e.line, Resolution: "declared"})
		}
	}
}

type ownedEvidence struct {
	owner, file string
	line        int
}

// pairs calls f once for each unordered pair of distinct owners, with the
// first evidence each owner gave, alphabetically first owner as `a`.
func (b *builder) pairs(ev []ownedEvidence, f func(a, c ownedEvidence)) {
	first := map[string]ownedEvidence{}
	for _, e := range ev {
		if have, ok := first[e.owner]; !ok || e.file < have.file || e.file == have.file && e.line < have.line {
			first[e.owner] = e
		}
	}
	owners := sortedKeys(first)
	for i := 0; i < len(owners); i++ {
		for j := i + 1; j < len(owners); j++ {
			f(first[owners[i]], first[owners[j]])
		}
	}
}

type dsKey struct{ vendor, host, db string }

func sortedDSKeys[V any](m map[dsKey]V) []dsKey {
	out := make([]dsKey, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].host != out[j].host {
			return out[i].host < out[j].host
		}
		if out[i].db != out[j].db {
			return out[i].db < out[j].db
		}
		return out[i].vendor < out[j].vendor
	})
	return out
}

var aspireDatastore = map[string]string{
	"Redis": "redis", "Garnet": "redis", "Valkey": "redis", "Postgres": "postgresql", "SqlServer": "sqlserver",
	"MySql": "mysql", "MongoDB": "mongodb", "Oracle": "oracle", "AzureCosmosDB": "cosmosdb", "Milvus": "milvus",
	"Qdrant": "qdrant", "Elasticsearch": "elasticsearch", "AzureStorage": "azure-storage", "AzureSqlServer": "sqlserver",
}

var aspireBroker = map[string]string{
	"RabbitMQ": "amqp", "Kafka": "kafka", "AzureServiceBus": "servicebus", "Nats": "nats", "AzureEventHubs": "eventhubs",
}

func (b *builder) aspireLinks(link func(*Link), datastore func(vendor, host, db, owner, file string, line int)) {
	projectDeployable := map[string]string{}
	for _, d := range b.m.Deployables {
		if d.BuiltBy == "aspire" && len(d.aliases) > 0 {
			projectDeployable[d.aliases[0]] = d.ID
		}
	}
	for _, host := range sortedKeys(b.aspire) {
		rs := b.aspire[host]
		byVar := map[string]*aspireResource{}
		for _, r := range rs {
			if r.Var != "" {
				byVar[r.Var] = r
			}
		}
		for _, r := range rs {
			owner := projectDeployable[r.Name]
			if r.Kind != "Project" || owner == "" {
				continue
			}
			for _, ref := range r.Refs {
				t := byVar[ref.Var]
				if t == nil {
					continue
				}
				switch {
				case t.Kind == "Project" && projectDeployable[t.Name] != "":
					link(&Link{From: owner, To: projectDeployable[t.Name], ToKind: "deployable", Kind: "calls", Mode: "sync", Via: t.Name, File: host, Line: ref.Line, Resolution: "declared"})
				case t.Kind == "Database":
					parent := byVar[t.Parent]
					vendor, server := "sql", t.Parent
					if parent != nil {
						server = parent.Name
						if v := aspireDatastore[parent.Kind]; v != "" {
							vendor = v
						}
					}
					link(&Link{From: owner, To: server + "/" + t.Name, ToKind: "external", Kind: "uses_datastore", Via: vendor, File: host, Line: ref.Line, Resolution: "declared"})
					datastore(vendor, server, strings.ToLower(t.Name), owner, host, ref.Line)
				case aspireDatastore[t.Kind] != "":
					link(&Link{From: owner, To: t.Name, ToKind: "external", Kind: "uses_datastore", Via: aspireDatastore[t.Kind], File: host, Line: ref.Line, Resolution: "declared"})
				case aspireBroker[t.Kind] != "":
					link(&Link{From: owner, To: t.Name, ToKind: "external", Kind: "messages", Mode: "async", Via: aspireBroker[t.Kind], File: host, Line: ref.Line, Resolution: "declared"})
				default:
					link(&Link{From: owner, To: t.Name, ToKind: "external", Kind: "calls", Mode: "sync", Via: t.Kind, File: host, Line: ref.Line, Resolution: "declared"})
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Pipelines and what they build and deploy
// ---------------------------------------------------------------------------

func (b *builder) pipelineRows() {
	for _, p := range b.pipelines {
		repo := b.in.RepoOf(p.File)
		row := &Pipeline{ID: p.ID, Name: p.Name, System: p.System, Kind: p.Kind, File: p.File, Repository: repo, Parsed: p.Parsed,
			Calls:    strings.Join(p.Calls, ", "),
			Triggers: strings.Join(p.Triggers, ", "), Paths: strings.Join(p.Paths, ", "),
			Stages: strings.Join(p.Stages(), ","), Tools: strings.Join(p.ToolList(), ", ")}
		var targets, refs []string
		for _, d := range p.Delegates {
			targets = append(targets, d.Target)
			refs = append(refs, d.Ref)
		}
		row.DelegatesTo = strings.Join(targets, ", ")
		row.DelegatesRef = strings.Join(refs, ", ")
		var envs []string
		for _, e := range p.Environments {
			envs = append(envs, e.Name)
		}
		row.Environments = strings.Join(envs, ", ")

		linked := map[string]bool{}
		add := func(id, action string, line int, resolution string) {
			k := id + "|" + action
			if id == "" || linked[k] {
				return
			}
			linked[k] = true
			b.m.PipelineDeployables = append(b.m.PipelineDeployables, &PipelineDeployable{Pipeline: p.ID, Deployable: id, Action: action, File: p.File, Line: line, Resolution: resolution})
		}
		images := p.Builds
		for _, u := range p.uses {
			images = append(images, u.Builds...)
		}
		for _, bd := range images {
			if d := b.buildOf[bd]; d != nil {
				add(d.ID, "builds", bd.Line, "declared")
			}
			if (bd.BuiltBy == "jib" || bd.BuiltBy == "skaffold") && bd.Context == "" {
				for _, d := range b.m.Deployables {
					if d.Repository != repo {
						continue
					}
					if bd.BuiltBy == "jib" && d.BuiltBy == "jib" || bd.BuiltBy == "skaffold" && d.priority == 1 {
						add(d.ID, "builds", bd.Line, "repository")
					}
				}
			}
		}
		for _, dh := range p.Deploys {
			for _, id := range b.deployTargets(dh, repo) {
				add(id, "deploys", dh.Line, "path")
			}
		}
		// A pipeline that builds or deploys without naming what: in a
		// repository that ships exactly one thing, it is that thing; in a
		// repository with several, what its path filter watches.
		builds := p.BuildTool || contains(p.Stages(), "package")
		deploys := contains(p.Stages(), "deploy")
		// An action runs only inside the workflows that use it, which are
		// credited with what it does.
		action := strings.HasSuffix(p.Kind, "_action") || p.Kind == "action"
		if (builds || deploys) && !action && !anyAction(linked, "builds") && !anyAction(linked, "deploys") {
			var ids []string
			resolution := "repository"
			if only := b.onlyDeployableIn(repo); only != "" {
				ids = []string{only}
			} else if len(p.Paths) > 0 {
				ids = b.deployablesUnder(p.Paths, repo)
				resolution = "paths"
			}
			for _, id := range ids {
				if builds {
					add(id, "builds", p.BuildToolLine, resolution)
				}
				if deploys {
					add(id, "deploys", p.stages["deploy"], resolution)
				}
			}
		}
		n := map[string]bool{}
		for k := range linked {
			n[strings.Split(k, "|")[0]] = true
		}
		row.Deployables = len(n)
		b.m.Pipelines = append(b.m.Pipelines, row)
	}
	// Delegated builds: the build is not done in this workspace.
	for _, pd := range b.m.PipelineDeployables {
		if pd.Action != "builds" {
			continue
		}
		d := b.byID[pd.Deployable]
		if d == nil || d.Kind != "app" {
			continue
		}
		for _, p := range b.pipelines {
			if p.ID == pd.Pipeline && len(p.Delegates) > 0 && !p.BuildToolLocal() {
				d.BuiltBy = "delegated"
			}
		}
	}
}

// BuildToolLocal reports whether the pipeline runs a build tool itself rather
// than in a delegated template.
func (p *pipelineFact) BuildToolLocal() bool {
	for _, d := range p.Delegates {
		if d.Line == p.BuildToolLine {
			return false
		}
	}
	return p.BuildTool
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func anyAction(linked map[string]bool, action string) bool {
	for k := range linked {
		if strings.HasSuffix(k, "|"+action) {
			return true
		}
	}
	return false
}

// deployTargets resolves what a deploy step names to deployables.
func (b *builder) deployTargets(dh deployHit, repo string) []string {
	var out []string
	under := func(p, target string) bool {
		return p == target || strings.HasPrefix(p, target+"/") || target == "."
	}
	switch dh.Kind {
	case "helm":
		for _, c := range b.charts {
			if under(c.Dir, dh.Target) || c.Dir == dh.Target {
				out = append(out, b.chartDeps[c.Dir]...)
			}
		}
	case "helm_values":
		for _, v := range b.values {
			if v.File == dh.Target {
				out = append(out, b.valuesOwners(v)...)
			}
		}
	case "kubectl", "kustomize":
		for _, f := range b.expandKustomize(dh.Target) {
			for _, w := range b.workloads {
				if under(w.File, f) {
					out = append(out, b.runs[w.Key]...)
				}
			}
		}
	case "skaffold":
		for _, d := range b.m.Deployables {
			if d.Repository == repo && (d.BuiltBy == "skaffold" || d.BuiltBy == "jib" || d.BuiltBy == "buildpacks" || d.BuiltBy == "ko") {
				out = append(out, d.ID)
			}
		}
	case "compose":
		for _, d := range b.m.Deployables {
			for _, bd := range d.builds {
				if bd.BuiltBy == "compose" && b.in.RepoOf(bd.File) == repo {
					out = append(out, d.ID)
				}
			}
		}
	case "sam":
		for _, d := range b.m.Deployables {
			if d.Kind == "function" && d.Repository == repo {
				out = append(out, d.ID)
			}
		}
	case "cloud":
		name := normalizeName(dh.Target)
		for _, d := range b.m.Deployables {
			if d.Name == name || contains(d.aliases, dh.Target) {
				out = append(out, d.ID)
			}
		}
	}
	sort.Strings(out)
	var uniq []string
	for _, id := range out {
		uniq = appendUnique(uniq, id)
	}
	return uniq
}

// expandKustomize follows a kustomization's resources down to files.
func (b *builder) expandKustomize(target string) []string {
	out := []string{target}
	seen := map[string]bool{}
	var walk func(dir string)
	walk = func(dir string) {
		if seen[dir] {
			return
		}
		seen[dir] = true
		for _, k := range b.kustomizes {
			if k.Dir != dir {
				continue
			}
			for _, r := range k.Resources {
				out = append(out, r)
				walk(r)
			}
		}
	}
	walk(target)
	return out
}

// deployablesUnder is the deployables holding a file some glob watches.
func (b *builder) deployablesUnder(globs []string, repo string) []string {
	var res []*regexp.Regexp
	for _, g := range globs {
		if strings.HasPrefix(g, "!") {
			continue
		}
		if re := globRegexp(g); re != nil {
			res = append(res, re)
		}
	}
	var out []string
	for _, d := range b.m.Deployables {
		if d.Repository != repo {
			continue
		}
	files:
		for _, c := range b.m.Contents {
			if c.Deployable != d.ID {
				continue
			}
			for _, f := range b.in.AllFiles {
				if !covers(c, f) {
					continue
				}
				for _, re := range res {
					if re.MatchString(f) {
						out = appendUnique(out, d.ID)
						break files
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// globRegexp turns a GitHub Actions path filter into a regular expression:
// `**` crosses folders, `*` does not.
func globRegexp(g string) *regexp.Regexp {
	g = strings.TrimPrefix(g, "./")
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch {
		case c == '*' && i+1 < len(g) && g[i+1] == '*':
			sb.WriteString(".*")
			i++
			if i+1 < len(g) && g[i+1] == '/' {
				i++
				sb.WriteString("/?")
			}
		case c == '*':
			sb.WriteString("[^/]*")
		case c == '?':
			sb.WriteString("[^/]")
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	sb.WriteString("$")
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil
	}
	return re
}

var envAffix = regexp.MustCompile(`(?i)([-_.](dev|develop|development|test|testing|qa|uat|stg|stage|staging|preprod|pre-prod|prod|production|nonprod|sandbox|local|int|integration|perf|demo)\d*)+$|^(dev|test|qa|uat|stg|staging|prod|nonprod)[-_.]`)

// stripEnvironment removes an environment written into a host name.
func stripEnvironment(host string) string {
	return envAffix.ReplaceAllString(host, "")
}
