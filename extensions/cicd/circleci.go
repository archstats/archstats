package cicd

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// CircleCI (tier 2). The graph is the workflows: each entry in a workflow's
// jobs list is a job, `requires` its needs, `type: approval` a gate. Steps
// come from the job's definition; orb jobs and commands are hand-offs to the
// orb at the version the orbs map pins.

func readCircleCI(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "circleci")
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Read = "none"
		p.unread(1, lineCount(content), "unsupported_syntax")
		return p
	}
	doc := docs[0]
	orbs := map[string]string{}
	for _, o := range pairs(get(doc, "orbs")) {
		orbs[o.Key] = str(o.Value)
	}
	defs := get(doc, "jobs")
	commands := get(doc, "commands")
	orbOf := func(name string) (string, string, bool) {
		alias, _, ok := strings.Cut(name, "/")
		if !ok {
			return "", "", false
		}
		ref, known := orbs[alias]
		if !known {
			return "", "", false
		}
		target, version, _ := strings.Cut(ref, "@")
		return target, version, true
	}
	var steps func(list []*yaml.Node, depth int)
	steps = func(list []*yaml.Node, depth int) {
		for _, st := range list {
			ln := line(st)
			if s := str(st); s != "" {
				r := p.step(s, s, ln)
				if s == "checkout" {
					r.Verb, r.Tool = "checkout", "git"
				}
				if target, version, ok := orbOf(s); ok {
					p.handoff("orb", target+" "+s, version, ln)
				} else if c := get(commands, s); c != nil && depth < 4 {
					steps(items(get(c, "steps")), depth+1)
				}
				continue
			}
			for _, kv := range pairs(st) {
				switch kv.Key {
				case "run":
					cmd := str(kv.Value)
					name := ""
					if cmd == "" {
						cmd = str(get(kv.Value, "command"))
						name = str(get(kv.Value, "name"))
					}
					p.step(name, cmd, ln)
					p.readShell(p.expand(cmd), ln, p.root)
				default:
					r := p.step(kv.Key, kv.Key, ln)
					if target, version, ok := orbOf(kv.Key); ok {
						p.handoff("orb", target+" "+kv.Key, version, ln)
					} else if c := get(commands, kv.Key); c != nil && depth < 4 {
						steps(items(get(c, "steps")), depth+1)
					}
					if kv.Key == "setup_remote_docker" {
						r.Verb = "setup"
					}
				}
			}
		}
	}
	runner := func(def *yaml.Node) string {
		if d := items(get(def, "docker")); len(d) > 0 {
			return str(get(d[0], "image"))
		}
		if m := get(def, "machine"); m != nil {
			if im := str(get(m, "image")); im != "" {
				return "machine · " + im
			}
			return "machine"
		}
		if e := str(get(def, "executor")); e != "" {
			return e
		}
		if mac := get(def, "macos"); mac != nil {
			return "macos " + str(get(mac, "xcode"))
		}
		return ""
	}
	for _, wf := range pairs(get(doc, "workflows")) {
		if wf.Key == "version" {
			continue
		}
		for _, t := range items(at(wf.Value, "triggers")) {
			p.trigger("schedule", str(at(t, "schedule", "cron")), line(t))
		}
		for _, entry := range items(get(wf.Value, "jobs")) {
			name, cfg := str(entry), (*yaml.Node)(nil)
			if name == "" {
				for _, kv := range pairs(entry) {
					name, cfg = kv.Key, kv.Value
				}
			}
			id := name
			if alias := str(get(cfg, "name")); alias != "" {
				id = alias
			}
			j := p.job(wf.Key+" › "+id, id, line(entry))
			j.Group = wf.Key
			for _, r := range strs(get(cfg, "requires")) {
				j.Needs = append(j.Needs, wf.Key+" › "+r)
			}
			if f := get(cfg, "filters"); f != nil {
				var parts []string
				if b := strs(at(f, "branches", "only")); len(b) > 0 {
					parts = append(parts, "branches: "+strings.Join(b, ", "))
					p.trigger("push", strings.Join(b, ", "), line(f))
				} else if b := str(at(f, "branches", "only")); b != "" {
					parts = append(parts, "branches: "+b)
					p.trigger("push", b, line(f))
				}
				if t := at(f, "tags", "only"); t != nil {
					parts = append(parts, "tags: "+firstNonEmpty(str(t), strings.Join(strs(t), ", ")))
					p.trigger("tag", firstNonEmpty(str(t), strings.Join(strs(t), ", ")), line(f))
				}
				j.Condition = strings.Join(parts, "; ")
			}
			if ctxs := strs(get(cfg, "context")); len(ctxs) > 0 {
				if env := envOfText(strings.Join(ctxs, " ")); env != "" {
					j.Environment = env
				}
			} else if c := str(get(cfg, "context")); c != "" {
				j.Environment = envOfText(c)
			}
			if str(get(cfg, "type")) == "approval" {
				j.Gate = "approval"
				s := p.step("approval", "type: approval", line(entry))
				s.Verb, s.sure = "approve", true
				continue
			}
			if target, version, ok := orbOf(name); ok {
				s := p.step("", "orb job "+name, line(entry))
				if v := verbOfTemplate(name); v != "other" {
					s.Verb = v
				}
				p.handoff("orb", target+" "+name, version, line(entry))
				continue
			}
			def := get(defs, name)
			if def == nil {
				p.unresolvedRef(name, line(entry), "job_not_found")
				continue
			}
			j.Runner = runner(def)
			if par := str(get(def, "parallelism")); par != "" {
				j.Matrix = atoiOr(par, 0)
			}
			steps(items(get(def, "steps")), 0)
		}
	}
	if len(p.Triggers) == 0 {
		p.trigger("push", "any branch", 1)
	}
	p.finish()
	return p
}
