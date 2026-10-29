package cicd

import (
	stdpath "path"
	"strings"

	"gopkg.in/yaml.v3"
)

// Azure Pipelines (tier 2): stages of jobs of steps, each level optional.
// A stage waits for the stage before it unless dependsOn says otherwise;
// jobs in a stage run side by side unless dependsOn says otherwise.
// Templates in this repository are read where they are used; templates in
// other repositories are hand-offs with the ref their resource names.

type azureReader struct {
	p     *pipelineFact
	rc    *readCtx
	repos map[string]string // resource alias -> ref
	depth int
}

func readAzurePipelines(file string, content []byte, rc *readCtx) *pipelineFact {
	p := newPipeline(file, "azure_pipelines")
	if rc == nil {
		rc = &readCtx{}
	}
	if rc.repo != "" {
		p.root = rc.repo
	}
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Read = "none"
		p.unread(1, lineCount(content), "unsupported_syntax")
		return p
	}
	doc := docs[0]
	r := &azureReader{p: p, rc: rc, repos: map[string]string{}}
	for _, res := range items(at(doc, "resources", "repositories")) {
		r.repos[str(get(res, "repository"))] = str(get(res, "ref"))
	}
	r.triggers(doc)
	for _, v := range r.variables(get(doc, "variables")) {
		p.vars[v[0]] = v[1]
	}
	if get(doc, "stages") == nil && get(doc, "jobs") == nil && get(doc, "steps") == nil && get(doc, "extends") == nil {
		// Only a template; read by whoever uses it.
		p.Kind = "template"
	}
	pool := poolOf(get(doc, "pool"))
	switch {
	case get(doc, "stages") != nil:
		r.stages(items(get(doc, "stages")), pool)
	case get(doc, "jobs") != nil:
		r.jobs(items(get(doc, "jobs")), "", nil, pool)
	case get(doc, "steps") != nil:
		j := p.job("job", "job", line(get(doc, "steps")))
		j.Runner = pool
		r.steps(items(get(doc, "steps")), p.root)
	case get(doc, "extends") != nil:
		t := str(at(doc, "extends", "template"))
		r.templateHandoff(t, line(get(doc, "extends")))
	}
	p.finish()
	return p
}

func (r *azureReader) triggers(doc *yaml.Node) {
	p := r.p
	branchFilter := func(n *yaml.Node) string {
		if n == nil {
			return ""
		}
		if n.Kind == yaml.SequenceNode {
			return strings.Join(strs(n), ", ")
		}
		var parts []string
		if inc := strs(at(n, "branches", "include")); len(inc) > 0 {
			parts = append(parts, "branches: "+strings.Join(inc, ", "))
		}
		if inc := strs(at(n, "tags", "include")); len(inc) > 0 {
			parts = append(parts, "tags: "+strings.Join(inc, ", "))
		}
		if inc := strs(at(n, "paths", "include")); len(inc) > 0 {
			parts = append(parts, "paths: "+strings.Join(inc, ", "))
			p.Paths = append(p.Paths, inc...)
		}
		return strings.Join(parts, "; ")
	}
	t := get(doc, "trigger")
	switch {
	case t == nil:
		p.trigger("push", "any branch", 1)
	case str(t) == "none":
	default:
		p.trigger("push", branchFilter(t), line(t))
	}
	if pr := get(doc, "pr"); pr == nil {
		p.trigger("pull_request", "", 1)
	} else if str(pr) != "none" {
		p.trigger("pull_request", branchFilter(pr), line(pr))
	}
	for _, s := range items(get(doc, "schedules")) {
		p.trigger("schedule", str(get(s, "cron")), line(s))
	}
	for _, rp := range items(at(doc, "resources", "pipelines")) {
		p.trigger("upstream", str(get(rp, "source")), line(rp))
	}
}

func (r *azureReader) variables(n *yaml.Node) [][2]string {
	var out [][2]string
	if n == nil {
		return nil
	}
	if n.Kind == yaml.MappingNode {
		for _, kv := range pairs(n) {
			out = append(out, [2]string{kv.Key, str(kv.Value)})
		}
		return out
	}
	for _, it := range items(n) {
		if name := str(get(it, "name")); name != "" {
			out = append(out, [2]string{name, str(get(it, "value"))})
		}
	}
	return out
}

func poolOf(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	if s := str(n); s != "" {
		return s
	}
	if v := str(get(n, "vmImage")); v != "" {
		return v
	}
	return str(get(n, "name"))
}

// stages reads a stages list; it returns nothing, jobs carry the order.
func (r *azureReader) stages(stages []*yaml.Node, pool string) {
	var prevStage []string
	stageJobs := map[string][]string{}
	for _, st := range stages {
		if t := str(get(st, "template")); t != "" {
			before := len(r.p.Jobs)
			r.template(t, line(st), "stages", pool)
			var ids []string
			for _, j := range r.p.Jobs[before:] {
				ids = append(ids, j.ID)
			}
			prevStage = ids
			continue
		}
		name := str(get(st, "stage"))
		var deps []string
		if d := get(st, "dependsOn"); d != nil {
			names := strs(d)
			if s := str(d); s != "" {
				names = []string{s}
			}
			for _, n := range names {
				deps = append(deps, stageJobs[n]...)
			}
		} else {
			deps = prevStage
		}
		stPool := pool
		if pl := poolOf(get(st, "pool")); pl != "" {
			stPool = pl
		}
		before := len(r.p.Jobs)
		r.jobs(items(get(st, "jobs")), name, deps, stPool)
		var ids []string
		for _, j := range r.p.Jobs[before:] {
			ids = append(ids, j.ID)
			if c := str(get(st, "condition")); c != "" && j.Condition == "" {
				j.Condition = oneLine(c, 200)
			}
		}
		stageJobs[name] = ids
		prevStage = ids
	}
}

func (r *azureReader) jobs(jobs []*yaml.Node, stage string, stageDeps []string, pool string) {
	p := r.p
	for _, jn := range jobs {
		if t := str(get(jn, "template")); t != "" {
			r.template(t, line(jn), "jobs", pool)
			continue
		}
		name := str(get(jn, "job"))
		deployment := false
		if name == "" {
			name = str(get(jn, "deployment"))
			deployment = name != ""
		}
		id := name
		if stage != "" {
			id = stage + "." + name
		}
		j := p.job(id, firstNonEmpty(str(get(jn, "displayName")), name), line(jn))
		j.Group = stage
		j.Runner = pool
		if pl := poolOf(get(jn, "pool")); pl != "" {
			j.Runner = pl
		}
		if c := str(get(jn, "container")); c != "" {
			j.Runner = strings.TrimPrefix(j.Runner+" · "+c, " · ")
		}
		if d := get(jn, "dependsOn"); d != nil {
			names := strs(d)
			if s := str(d); s != "" {
				names = []string{s}
			}
			for _, n := range names {
				if stage != "" {
					n = stage + "." + n
				}
				j.Needs = append(j.Needs, n)
			}
		} else {
			j.Needs = append(j.Needs, stageDeps...)
		}
		j.Condition = oneLine(str(get(jn, "condition")), 200)
		if m := get(jn, "strategy"); m != nil {
			if mx := get(m, "matrix"); mx != nil {
				j.Matrix = len(pairs(mx))
			} else if n := str(get(m, "parallel")); n != "" {
				j.Matrix = atoiOr(n, -1)
			}
		}
		if deployment {
			env := get(jn, "environment")
			name := str(env)
			if name == "" {
				name = str(get(env, "name"))
			}
			j.Environment, j.Gate = name, "environment"
			for _, strat := range pairs(get(jn, "strategy")) {
				for _, phase := range pairs(strat.Value) {
					r.steps(items(at(phase.Value, "steps")), ".")
					for _, sub := range pairs(phase.Value) {
						if sub.Key != "steps" {
							r.steps(items(get(sub.Value, "steps")), ".")
						}
					}
				}
			}
		}
		r.steps(items(get(jn, "steps")), ".")
	}
}

func atoiOr(s string, def int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

func (r *azureReader) steps(steps []*yaml.Node, dir string) {
	p := r.p
	for _, st := range steps {
		ln := line(st)
		name := str(get(st, "displayName"))
		switch {
		case get(st, "template") != nil:
			r.template(str(get(st, "template")), ln, "steps", "")
		case get(st, "checkout") != nil:
			s := p.step(name, "checkout "+str(get(st, "checkout")), ln)
			s.Verb, s.Tool = "checkout", "git"
		case get(st, "task") != nil:
			task := str(get(st, "task"))
			in := get(st, "inputs")
			text := task
			if c := str(get(in, "command")); c != "" {
				text += " " + c
			}
			s := p.step(name, text, ln)
			r.task(s, task, in, ln)
		default:
			for _, k := range []string{"script", "bash", "powershell", "pwsh"} {
				if v := str(get(st, k)); v != "" {
					p.step(name, v, ln)
					wd := str(get(st, "workingDirectory"))
					p.readShell(p.expand(v), ln, joinPath(dir, strings.TrimPrefix(wd, "$(System.DefaultWorkingDirectory)/")))
				}
			}
			if get(st, "publish") != nil || get(st, "download") != nil {
				p.step(name, "artifact", ln).Verb = "other"
			}
		}
	}
}

// task reads the tasks that build, push and deploy.
func (r *azureReader) task(s *stepFact, task string, in *yaml.Node, ln int) {
	p := r.p
	kind, _, _ := strings.Cut(task, "@")
	cmd := strings.ToLower(str(get(in, "command")))
	switch kind {
	case "Docker":
		if cmd == "" || strings.Contains(cmd, "build") {
			df := firstNonEmpty(str(get(in, "Dockerfile")), str(get(in, "dockerfile")), "Dockerfile")
			ctx := firstNonEmpty(str(get(in, "buildContext")), dirOf(strings.TrimPrefix(df, "**/")))
			b := &imageBuild{BuiltBy: "pipeline", File: p.File, Line: ln, Context: joinPath(p.root, strings.TrimPrefix(ctx, "$(Build.SourcesDirectory)/")), Dockerfile: joinPath(p.root, strings.TrimPrefix(df, "**/"))}
			if repo := p.expand(str(get(in, "repository"))); repo != "" {
				b.Names = append(b.Names, NormalizeImage(repo))
				for _, t := range splitList(str(get(in, "tags"))) {
					b.RawTags = append(b.RawTags, repo+":"+t)
				}
			}
			p.addBuild(b)
		}
		if strings.Contains(cmd, "push") {
			s.Verb = "publish"
		}
	case "KubernetesManifest", "Kubernetes":
		for _, m := range splitList(str(get(in, "manifests"))) {
			p.addDeploy(&deployHit{Kind: "kubectl", Target: joinPath(p.root, m), Line: ln})
		}
	case "HelmDeploy":
		if c := str(get(in, "chartPath")); c != "" {
			p.addDeploy(&deployHit{Kind: "helm", Target: joinPath(p.root, c), Line: ln})
		}
	case "AzureWebApp", "AzureFunctionApp", "AzureRmWebAppDeployment", "AzureContainerApps":
		p.addDeploy(&deployHit{Kind: "cloud", Target: firstNonEmpty(str(get(in, "appName")), str(get(in, "WebAppName")), str(get(in, "containerAppName"))), Line: ln})
	case "ManualValidation":
		s.Verb, s.sure = "approve", true
		if p.cj != nil {
			p.cj.Gate = "approval"
		}
	case "Maven", "Gradle", "DotNetCoreCLI", "Npm":
		if !p.BuildTool {
			p.BuildTool, p.BuildToolLine, p.buildToolJob = true, ln, p.jobID()
		}
	}
}

// template reads a template: in this repository, inline; in another, as a
// hand-off.
func (r *azureReader) template(t string, ln int, level, pool string) {
	if t == "" {
		return
	}
	if strings.Contains(t, "@") && !strings.HasSuffix(t, "@self") {
		r.templateHandoff(t, ln)
		return
	}
	t = strings.TrimSuffix(t, "@self")
	path := joinPath(dirOf(r.p.File), t)
	if strings.HasPrefix(t, "/") && r.rc.repo != "" {
		path = joinPath(r.rc.repo, strings.TrimPrefix(t, "/"))
	}
	content, ok := []byte(nil), false
	if r.rc.lookup != nil {
		content, ok = r.rc.lookup(path)
	}
	if !ok || r.depth > 5 {
		h := r.p.handoff("template", path, "", ln)
		h.Pinned = ""
		r.p.unresolvedRef(t, ln, "template_not_found")
		return
	}
	h := r.p.handoff("template", path, "", ln)
	h.Pinned, h.Local = "", path
	docs := yamlDocs(content)
	if len(docs) == 0 {
		return
	}
	r.depth++
	saved := r.p.curFile
	r.p.curFile = path
	doc := docs[0]
	switch {
	case get(doc, "stages") != nil:
		r.stages(items(get(doc, "stages")), pool)
	case get(doc, "jobs") != nil:
		r.jobs(items(get(doc, "jobs")), "", nil, pool)
	case get(doc, "steps") != nil:
		r.steps(items(get(doc, "steps")), r.p.root)
	}
	r.p.curFile = saved
	r.depth--
	_ = level
}

func (r *azureReader) templateHandoff(t string, ln int) {
	target, alias, _ := strings.Cut(t, "@")
	ref := r.repos[alias]
	r.p.handoff("template", alias+"/"+strings.TrimPrefix(stdpath.Clean("/"+target), "/"), strings.TrimPrefix(strings.TrimPrefix(ref, "refs/tags/"), "refs/heads/"), ln)
}
