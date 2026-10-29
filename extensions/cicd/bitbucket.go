package cicd

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Bitbucket Pipelines (tier 2). Each section under pipelines: (default, a
// branch, a pull request glob, a tag, a custom pipeline) is a flow of its
// own: steps run in order, a parallel block side by side, a stage's steps
// in order under one deployment.

func readBitbucket(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "bitbucket")
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Read = "none"
		p.unread(1, lineCount(content), "unsupported_syntax")
		return p
	}
	doc := docs[0]
	image := str(get(doc, "image"))
	if image == "" {
		image = str(at(doc, "image", "name"))
	}
	pipes := get(doc, "pipelines")
	flow := func(group string, list *yaml.Node) {
		var prev []string
		n := 0
		var step func(st *yaml.Node, needs []string, deployment string) string
		step = func(st *yaml.Node, needs []string, deployment string) string {
			n++
			name := str(get(st, "name"))
			if name == "" {
				name = "step " + itoa(n)
			}
			j := p.job(group+" › "+name, name, line(st))
			j.Group = group
			j.Needs = append(j.Needs, needs...)
			j.Runner = image
			if im := str(get(st, "image")); im != "" {
				j.Runner = im
			} else if im := str(at(st, "image", "name")); im != "" {
				j.Runner = im
			}
			if svc := strs(get(st, "services")); len(svc) > 0 {
				j.Runner = strings.TrimPrefix(j.Runner+" · "+strings.Join(svc, ", "), " · ")
			}
			env := str(get(st, "deployment"))
			if env == "" {
				env = deployment
			}
			if env != "" {
				j.Environment, j.Gate = env, "environment"
			}
			if str(get(st, "trigger")) == "manual" {
				j.Gate = "manual"
			}
			if cond := strs(at(st, "condition", "changesets", "includePaths")); len(cond) > 0 {
				j.Condition = "changes: " + strings.Join(cond, ", ")
			}
			for _, sc := range items(get(st, "script")) {
				if pipe := str(get(sc, "pipe")); pipe != "" {
					target, ref, _ := strings.Cut(pipe, ":")
					s := p.step("pipe", "pipe: "+pipe, line(sc))
					if v, _ := classifyVerb(target); v != "" {
						s.Verb = v
					}
					s.Target = target
					p.handoff("pipe", target, ref, line(sc))
					continue
				}
				cmd := str(sc)
				if cmd == "" {
					continue
				}
				p.step("", cmd, line(sc))
				p.readShell(p.expand(cmd), line(sc), p.root)
			}
			for _, sc := range items(get(st, "after-script")) {
				if cmd := str(sc); cmd != "" {
					p.step("after-script", cmd, line(sc))
				}
			}
			return j.ID
		}
		for _, it := range items(list) {
			switch {
			case get(it, "step") != nil:
				prev = []string{step(get(it, "step"), prev, "")}
			case get(it, "parallel") != nil:
				par := get(it, "parallel")
				steps := items(par)
				if len(steps) == 0 {
					steps = items(get(par, "steps"))
				}
				var ends []string
				for _, ps := range steps {
					if s := get(ps, "step"); s != nil {
						ends = append(ends, step(s, prev, ""))
					}
				}
				if len(ends) > 0 {
					prev = ends
				}
			case get(it, "stage") != nil:
				stg := get(it, "stage")
				dep := str(get(stg, "deployment"))
				for _, ss := range items(get(stg, "steps")) {
					if s := get(ss, "step"); s != nil {
						prev = []string{step(s, prev, dep)}
					}
				}
			}
		}
	}
	for _, sec := range pairs(pipes) {
		switch sec.Key {
		case "default":
			p.trigger("push", "any branch without its own pipeline", line(sec.KeyN))
			flow("default", sec.Value)
		case "branches", "pull-requests", "tags", "custom":
			for _, b := range pairs(sec.Value) {
				switch sec.Key {
				case "branches":
					p.trigger("push", b.Key, line(b.KeyN))
				case "pull-requests":
					p.trigger("pull_request", b.Key, line(b.KeyN))
				case "tags":
					p.trigger("tag", b.Key, line(b.KeyN))
				case "custom":
					p.trigger("manual", b.Key, line(b.KeyN))
				}
				flow(sec.Key+": "+b.Key, b.Value)
			}
		}
	}
	p.finish()
	return p
}
