package deployables

import (
	stdpath "path"
	"regexp"
	"strings"
)

// A deployable is built once and runs in several places. A Helm chart and a
// values file per environment is the common shape, and the one the client
// workspace uses for all 20 of its services: helm-release/nonprod-dev.yaml,
// nonprod-test, nonprod-staging and prod, each configuring a chart that lives
// outside the workspace. Some places are open-ended -- a preview per pull
// request -- and are recorded as one pattern, never enumerated.

var (
	valuesEnvName = regexp.MustCompile(`^values[._-](.+)\.ya?ml$|^(.+)[._-]values\.ya?ml$`)
	overlaySeg    = regexp.MustCompile(`(?:^|/)(?:overlays|envs|environments|environment|stages|clusters)/([^/]+)(?:/|$)`)
	releaseDir    = regexp.MustCompile(`(?i)(^|/)(helm[-_]?release[s]?|releases?|environments?|envs?|deploy(ments?)?|values)(/|$)`)
)

// valuesEnvironment reads the environment a values file configures, or ""
// for a chart's base values.yaml and for files that name none.
func valuesEnvironment(file string, chartBeside bool) string {
	base := stdpath.Base(file)
	lower := strings.ToLower(base)
	if lower == "values.yaml" || lower == "values.yml" {
		return ""
	}
	if m := valuesEnvName.FindStringSubmatch(lower); m != nil {
		if m[1] != "" {
			return base[len("values."):strings.LastIndex(base, ".")]
		}
		return base[:len(m[2])]
	}
	if !chartBeside && releaseDir.MatchString(dirOf(file)) {
		return strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
	}
	return ""
}

func (b *builder) environments() {
	add := func(deployable, env, kind, source, file string, line int) {
		if deployable == "" || env == "" {
			return
		}
		for _, e := range b.m.Environments {
			if e.Deployable == deployable && e.Environment == env && e.Source == source {
				return
			}
		}
		b.m.Environments = append(b.m.Environments, &Environment{Deployable: deployable, Environment: env, Kind: kind, Source: source, File: file, Line: line})
	}
	chartDirs := map[string]bool{}
	for _, c := range b.charts {
		chartDirs[c.Dir] = true
	}
	// Helm values per environment, with every setting kept for the diff.
	for _, v := range b.values {
		env := valuesEnvironment(v.File, chartDirs[v.Dir])
		if env == "" {
			continue
		}
		secretItem := secretListItems(v.Flat)
		keyed := keyListsByName(v.Flat)
		for _, id := range b.valuesOwners(v) {
			add(id, env, "enumerated", "helm_values", v.File, 1)
			for i, fv := range v.Flat {
				if keyed[i] == "" {
					continue
				}
				original := fv.Key
				fv.Key = keyed[i]
				ev := &EnvValue{Deployable: id, Environment: env, Source: "helm_values", Key: fv.Key, Value: fv.Value, File: v.File, Line: fv.Line}
				if isSecretKey(fv.Key) || secretItem[original] {
					ev.Value, ev.Secret = "", 1
				} else {
					ev.Value = redactURL(fv.Value)
				}
				b.m.EnvValues = append(b.m.EnvValues, ev)
			}
		}
	}
	// Kustomize overlays: overlays/prod is prod.
	for _, k := range b.kustomizes {
		m := overlaySeg.FindStringSubmatch(k.Dir + "/")
		if m == nil {
			continue
		}
		env := m[1]
		for _, id := range b.deployTargets(deployHit{Kind: "kustomize", Target: k.Dir}, "") {
			add(id, env, "enumerated", "kustomize_overlay", k.File, 1)
		}
		idx := b.imageIndex()
		for _, im := range k.Images {
			if l := idx.findImage(NormalizeImage(im.NewName)); l.found() {
				add(l.ID, env, "enumerated", "kustomize_overlay", k.File, im.Line)
			}
		}
	}
	// Where pipelines deploy.
	for _, p := range b.pipelines {
		if len(p.Environments) == 0 {
			continue
		}
		var targets []string
		for _, pd := range b.m.PipelineDeployables {
			if pd.Pipeline == p.ID && pd.Action == "deploys" {
				targets = appendUnique(targets, pd.Deployable)
			}
		}
		for _, id := range targets {
			for _, e := range p.Environments {
				add(id, e.Name, e.Kind, "workflow", p.File, e.Line)
			}
		}
	}
	// Argo CD: an ApplicationSet enumerates, or generates without limit.
	for _, a := range b.argo {
		if a.Path == "" || strings.Contains(a.Path, "{{") {
			continue
		}
		ids := b.deployTargets(deployHit{Kind: "kustomize", Target: joinPath(".", a.Path)}, "")
		for _, c := range b.charts {
			if c.Dir == joinPath(".", a.Path) {
				ids = append(ids, b.chartDeps[c.Dir]...)
			}
		}
		for _, id := range ids {
			for _, el := range a.Elements {
				add(id, el, "enumerated", "applicationset", a.File, a.Line)
			}
			for _, pat := range a.Patterns {
				add(id, pat, "pattern", "applicationset", a.File, a.Line)
			}
		}
	}
	// Spring profiles: application-prod.yml configures the prod profile.
	for _, f := range b.configFiles {
		profile := springProfileOf(f)
		if profile == "" || strings.EqualFold(profile, "test") || strings.HasPrefix(stdpath.Base(f), ".env") {
			continue
		}
		for _, id := range b.ownersOfFile(f) {
			add(id, profile, "enumerated", "config_profile", f, 1)
		}
	}
}

// redactURL keeps a URL's scheme, host, port and path and drops any user or
// password written into it.
func redactURL(v string) string {
	if i := strings.Index(v, "://"); i >= 0 {
		rest := v[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 && (strings.Index(rest, "/") < 0 || at < strings.Index(rest, "/")) {
			return v[:i+3] + rest[at+1:]
		}
	}
	return v
}

// secretListItems finds the values of `- name: DB_PASSWORD  value: ...`
// list items, whose key says nothing and whose sibling says everything.
func secretListItems(flat []flatValue) map[string]bool {
	out := map[string]bool{}
	names := map[string]string{}
	for _, fv := range flat {
		if strings.HasSuffix(fv.Key, ".name") {
			names[strings.TrimSuffix(fv.Key, ".name")] = fv.Value
		}
	}
	for _, fv := range flat {
		if strings.HasSuffix(fv.Key, ".value") {
			if isSecretKey(names[strings.TrimSuffix(fv.Key, ".value")]) {
				out[fv.Key] = true
			}
		}
	}
	return out
}

// listEnvEntries turns `- name: K  value: V` items in flattened values into
// K=V entries, so a Helm `env:` list is read like any environment.
func listEnvEntries(flat []flatValue, file string) []configEntry {
	names := map[string]flatValue{}
	for _, fv := range flat {
		if strings.HasSuffix(fv.Key, ".name") {
			names[strings.TrimSuffix(fv.Key, ".name")] = fv
		}
	}
	var out []configEntry
	for _, fv := range flat {
		if !strings.HasSuffix(fv.Key, ".value") {
			continue
		}
		if n, ok := names[strings.TrimSuffix(fv.Key, ".value")]; ok && n.Value != "" {
			out = append(out, configEntry{Key: n.Value, Value: fv.Value, File: file, Line: fv.Line})
		}
	}
	return out
}

var listIndex = regexp.MustCompile(`^(.*)\.(\d+)$`)

// keyListsByName rewrites the keys of list items that carry a name --
// `extraVars.3.value` becomes `extraVars[DB_URL].value` -- so two
// environments that list the same variables in a different order compare
// variable by variable, not position by position. The `.name` entry itself
// is dropped (returned as ""). Other keys are returned unchanged.
func keyListsByName(flat []flatValue) []string {
	names := map[string]string{} // "extraVars.3" -> "extraVars[DB_URL]"
	for _, fv := range flat {
		if !strings.HasSuffix(fv.Key, ".name") || fv.Value == "" {
			continue
		}
		item := strings.TrimSuffix(fv.Key, ".name")
		if m := listIndex.FindStringSubmatch(item); m != nil {
			names[item] = m[1] + "[" + fv.Value + "]"
		}
	}
	out := make([]string, len(flat))
	for i, fv := range flat {
		out[i] = fv.Key
		for item, named := range names {
			if fv.Key == item+".name" {
				out[i] = ""
				break
			}
			if strings.HasPrefix(fv.Key, item+".") {
				out[i] = named + fv.Key[len(item):]
				break
			}
		}
	}
	return out
}
