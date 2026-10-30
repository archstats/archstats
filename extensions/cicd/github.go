package cicd

import (
	stdpath "path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// GitHub Actions: workflows, reusable workflows and the actions a repository
// defines. Jobs and their needs are written out; steps are read for what
// they run, and `uses:` is a hand-off with the ref it is pinned to.

var (
	envInputRef  = regexp.MustCompile(`(?i)inputs\.([a-z0-9_-]+)`)
	versionInput = regexp.MustCompile(`(?i)^(java|node|go|python|dotnet|jdk)[-_]?version$`)
	envInputName = regexp.MustCompile(`(?i)^(env|environment|stage|target[-_]?env|deploy[-_]?env|environment[-_]?name)$`)
	prHeadRef    = regexp.MustCompile(`github\.event\.pull_request\.head|github\.head_ref|refs/pull/`)
)

func readGitHubWorkflow(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "github_actions")
	p.Kind = "workflow"
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Read = "none"
		p.unread(1, lineCount(content), "unsupported_syntax")
		return p
	}
	doc := docs[0]
	if n := str(get(doc, "name")); n != "" {
		p.Name = n
	}
	// `on` is read by YAML 1.1 parsers as true; yaml.v3 keeps the key text.
	on := get(doc, "on")
	if on == nil {
		on = get(doc, "true")
	}
	prTarget := false
	// A workflow only ever called runs with its caller's token, so its
	// caller's permissions are the ones that count.
	events, called := 0, 0
	ghEvent := func(name string, v *yaml.Node, ln int) {
		events++
		var filter []string
		for _, k := range []string{"branches", "branches-ignore", "tags", "tags-ignore", "paths", "paths-ignore", "types", "workflows"} {
			if vs := strs(get(v, k)); len(vs) > 0 {
				filter = append(filter, k+": "+strings.Join(vs, ", "))
			}
		}
		for _, g := range strs(get(v, "paths")) {
			p.Paths = append(p.Paths, g)
		}
		for _, g := range strs(get(v, "paths-ignore")) {
			p.Paths = append(p.Paths, "!"+g)
		}
		f := strings.Join(filter, "; ")
		switch name {
		case "push":
			if len(strs(get(v, "tags"))) > 0 && len(strs(get(v, "branches"))) == 0 {
				p.trigger("tag", f, ln)
			} else {
				p.trigger("push", f, ln)
			}
		case "pull_request", "pull_request_target", "merge_group":
			if name != "pull_request" {
				f = strings.TrimPrefix(name+"; "+f, "; ")
				f = strings.TrimSuffix(f, "; ")
			}
			if name == "pull_request_target" {
				prTarget = true
			}
			p.trigger("pull_request", f, ln)
		case "schedule":
			for _, it := range items(v) {
				p.trigger("schedule", str(get(it, "cron")), line(it))
			}
		case "workflow_dispatch":
			p.trigger("manual", "", ln)
		case "workflow_call":
			called++
			p.Kind = "reusable_workflow"
			p.trigger("called", "", ln)
		case "workflow_run":
			p.trigger("upstream", f, ln)
		case "release":
			p.trigger("tag", strings.TrimPrefix("release; "+f, ""), ln)
		default:
			p.trigger("other", name, ln)
		}
	}
	switch {
	case on == nil:
	case on.Kind == yaml.ScalarNode:
		ghEvent(str(on), nil, line(on))
	case on.Kind == yaml.SequenceNode:
		for _, it := range items(on) {
			ghEvent(str(it), nil, line(it))
		}
	case on.Kind == yaml.MappingNode:
		for _, t := range pairs(on) {
			ghEvent(t.Key, t.Value, line(t.KeyN))
		}
	}
	dispatchEnvs := map[string][]string{}
	for _, in := range pairs(at(on, "workflow_dispatch", "inputs")) {
		if opts := strs(get(in.Value, "options")); envInputName.MatchString(in.Key) && len(opts) > 0 {
			dispatchEnvs[in.Key] = opts
		}
	}
	for _, e := range envEntries(get(doc, "env")) {
		p.vars[e.Key] = e.Value
	}
	if get(doc, "permissions") != nil {
		p.permissions = true
	}
	jobs := pairs(get(doc, "jobs"))
	allJobsPermitted := len(jobs) > 0
	for _, job := range jobs {
		j := job.Value
		jf := p.job(job.Key, str(get(j, "name")), line(job.KeyN))
		if strings.Contains(jf.Name, "${{") {
			jf.Name = job.Key
		}
		if get(j, "permissions") == nil {
			allJobsPermitted = false
		}
		needs := get(j, "needs")
		if needs != nil && needs.Kind == yaml.ScalarNode {
			jf.Needs = []string{str(needs)}
		} else {
			jf.Needs = strs(needs)
		}
		jf.Runner = runnerOf(get(j, "runs-on"))
		if c := get(j, "container"); c != nil {
			img := str(c)
			if img == "" {
				img = str(get(c, "image"))
			}
			if img != "" {
				jf.Runner = strings.TrimPrefix(jf.Runner+" · "+img, " · ")
			}
		}
		jf.Condition = oneLine(str(get(j, "if")), 200)
		jobEnv := map[string]string{}
		for k, v := range p.vars {
			jobEnv[k] = v
		}
		for _, e := range envEntries(get(j, "env")) {
			jobEnv[e.Key] = e.Value
		}
		combos := matrixCombos(at(j, "strategy", "matrix"))
		jf.Matrix = len(combos)
		if m := at(j, "strategy", "matrix"); m != nil && m.Kind == yaml.ScalarNode {
			// A matrix from an expression (fromJSON of an earlier job's
			// output): there is one, its size is not in the file.
			jf.Matrix = -1
		}
		if env := get(j, "environment"); env != nil {
			name := str(env)
			if name == "" {
				name = str(get(env, "name"))
			}
			if m := envInputRef.FindStringSubmatch(name); m != nil && len(dispatchEnvs[m[1]]) > 0 {
				name = strings.Join(dispatchEnvs[m[1]], " | ")
			} else if strings.Contains(name, "${{") && len(combos) > 0 {
				var names []string
				for _, c := range combos {
					if v := expandGH(name, c, jobEnv); !strings.Contains(v, "${{") {
						names = appendUnique(names, v)
					}
				}
				if len(names) > 0 {
					name = strings.Join(names, " | ")
				}
			}
			jf.Environment, jf.Gate = name, "environment"
		}
		// A reusable workflow: the job is the call.
		if uses := localUses(str(get(j, "uses"))); uses != "" {
			ln := line(get(j, "uses"))
			target, ref, _ := strings.Cut(uses, "@")
			s := p.step("", "uses: "+uses, ln)
			if v := verbOfTemplate(target); v != "other" {
				s.Verb = v
			}
			s.Target = target
			if s.Verb == "build" || s.Verb == "package" || s.Verb == "test" {
				if !p.BuildTool {
					p.BuildTool, p.BuildToolLine, p.buildToolJob = true, ln, jf.ID
				}
				if !strings.HasPrefix(uses, "./") {
					p.buildDelegated = true
				}
			}
			if strings.HasPrefix(uses, "./") {
				local := joinPath(p.root, strings.TrimPrefix(target, "./"))
				p.call(local, ln)
				p.handoff("reusable_workflow", local, "", ln).Pinned = ""
			} else {
				p.handoff("reusable_workflow", target, ref, ln)
				if strings.Contains(target, "/.github/workflows/") || strings.Count(target, "/") >= 2 {
					p.deployByName(s, target, ln)
				}
			}
			for _, w := range pairs(get(j, "with")) {
				if versionInput.MatchString(w.Key) && str(w.Value) != "" {
					p.Runtimes = append(p.Runtimes, runtimeHit{Tool: runtimeTool(w.Key), Version: str(w.Value), Line: line(w.KeyN)})
				}
			}
			continue
		}
		workdir := str(at(j, "defaults", "run", "working-directory"))
		p.readGitHubSteps(items(get(j, "steps")), combos, jobEnv, workdir, prTarget && !botAuthorOnly.MatchString(str(get(j, "if"))))
	}
	if allJobsPermitted {
		p.permissions = true
	}
	if !p.permissions && len(p.Jobs) > 0 && (events == 0 || called < events) {
		// About the workflow, not the job read last.
		p.cj = nil
		p.flag("no_permissions_block", "", "", 1)
	}
	p.finish()
	return p
}

// deployByName credits a deploy to a delegated template whose name says it
// deploys: deployment-nonproduction.yml, deploy-to-k8s.
func (p *pipelineFact) deployByName(s *stepFact, target string, ln int) {
	if verbOfTemplate(target) == "deploy" {
		p.addDeploy(&deployHit{Kind: "delegated", Platform: "other", Target: target, Line: ln})
		s.Verb = "deploy"
	}
}

// A job that runs only for pull requests a bot opened: the code it checks
// out is the bot's, which is the guard pull_request_target needs. The
// author cannot be spoofed the way github.actor can.
var botAuthorOnly = regexp.MustCompile(`^[^|]*github\.event\.pull_request\.user\.login\s*==\s*'[A-Za-z0-9-]+\[bot\]'[^|]*$`)

func runnerOf(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return str(n)
	case yaml.SequenceNode:
		return strings.Join(strs(n), ", ")
	case yaml.MappingNode:
		var parts []string
		if g := str(get(n, "group")); g != "" {
			parts = append(parts, "group "+g)
		}
		parts = append(parts, strs(get(n, "labels"))...)
		return strings.Join(parts, ", ")
	}
	return ""
}

var nodeRuntime = regexp.MustCompile(`^node(\d+)$`)

// readGitHubAction reads an action this workspace defines (action.yml). A
// composite action is steps like a job's, and what they do counts in every
// workflow that uses it; a Docker action runs an image; a JavaScript action
// runs a script on the runner's Node. None of them starts on its own.
func readGitHubAction(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "github_actions")
	p.Kind = "action"
	if dir := dirOf(file); dir != "." {
		p.Name = stdpath.Base(dir)
	}
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Read = "none"
		p.unread(1, lineCount(content), "unsupported_syntax")
		return p
	}
	doc := docs[0]
	if n := str(get(doc, "name")); n != "" {
		p.Name = n
	}
	p.trigger("called", "", 1)
	runs := get(doc, "runs")
	using := strings.ToLower(str(get(runs, "using")))
	ln := line(get(runs, "using"))
	p.job(p.Name, p.Name, ln)
	switch {
	case using == "composite":
		p.readGitHubSteps(items(get(runs, "steps")), nil, map[string]string{}, "", false)
	case using == "docker":
		image := str(get(runs, "image"))
		s := p.step("runs "+image, "docker run "+image, line(get(runs, "image")))
		s.Tool = "docker"
		p.cj.Runner = "docker"
	case nodeRuntime.MatchString(using):
		s := p.step(str(get(runs, "main")), "node "+str(get(runs, "main")), ln)
		s.Tool = "npm"
		p.cj.Runner = using
		p.Runtimes = append(p.Runtimes, runtimeHit{Tool: "node", Version: nodeRuntime.FindStringSubmatch(using)[1], Line: ln})
	default:
		p.unread(ln, ln, "unsupported_syntax")
	}
	p.finish()
	return p
}

// readGitHubSteps reads the steps of a job or of a composite action: what
// each runs, the images it builds and what it deploys, once per matrix
// combination.
func (p *pipelineFact) readGitHubSteps(steps []*yaml.Node, combos []map[string]interface{}, jobEnv map[string]string, workdir string, prTarget bool) {
	if len(combos) == 0 {
		combos = []map[string]interface{}{nil}
	}
	for _, st := range steps {
		ln := line(st)
		uses := localUses(str(get(st, "uses")))
		run := str(get(st, "run"))
		name := str(get(st, "name"))
		dir := joinPath(p.root, workdir)
		if wd := str(get(st, "working-directory")); wd != "" {
			dir = joinPath(p.root, wd)
		}
		if dir == "" {
			dir = "."
		}
		text := run
		if uses != "" {
			text = "uses: " + uses
		}
		s := p.step(expandGH(name, combos[0], jobEnv), text, ln)
		with := get(st, "with")
		if uses != "" {
			target, ref, _ := strings.Cut(uses, "@")
			// The action's own name says what a step that uses it does,
			// better than the step's label.
			if v, sure := classifyVerb(target); v != "" && (sure || s.Verb == "other") {
				s.Verb = v
			}
			if strings.HasPrefix(target, "docker://") {
				p.handoff("action", target, "", ln).Pinned = pinOfImage(target)
			} else if strings.HasPrefix(uses, "./") {
				local := joinPath(p.root, strings.TrimPrefix(target, "./"))
				p.call(local, ln)
				p.handoff("action", local, "", ln).Pinned = ""
			} else {
				p.handoff("action", target, ref, ln)
			}
			if strings.HasPrefix(uses, "actions/checkout") && prTarget {
				if prHeadRef.MatchString(str(get(with, "ref"))) {
					p.flag("pr_target_checkout_head", "pull_request_target checks out "+str(get(with, "ref")), "", ln)
				}
			}
		}
		for _, w := range pairs(with) {
			if versionInput.MatchString(w.Key) && str(w.Value) != "" && !strings.Contains(str(w.Value), "${{") {
				p.Runtimes = append(p.Runtimes, runtimeHit{Tool: runtimeTool(w.Key + " " + uses), Version: str(w.Value), Line: line(w.KeyN)})
			}
		}
		for _, combo := range combos {
			if run != "" {
				p.readShell(expandGH(run, combo, jobEnv), ln, dir)
			}
			switch {
			case strings.HasPrefix(uses, "docker/build-push-action"):
				b := &imageBuild{BuiltBy: "pipeline", File: p.File, Line: ln}
				ctx := expandGH(str(get(with, "context")), combo, jobEnv)
				if ctx == "" {
					ctx = "."
				}
				if isInterpolated(ctx) {
					continue
				}
				b.Context = joinPath(p.root, ctx)
				df := expandGH(str(get(with, "file")), combo, jobEnv)
				if df == "" {
					df = stdpath.Join(ctx, "Dockerfile")
				}
				b.Dockerfile = joinPath(p.root, df)
				b.Target = expandGH(str(get(with, "target")), combo, jobEnv)
				for _, t := range splitList(expandGH(str(get(with, "tags")), combo, jobEnv)) {
					b.Names = append(b.Names, NormalizeImage(t))
					b.RawTags = append(b.RawTags, t)
				}
				// Tags from docker/metadata-action name the image in its `images`.
				for _, other := range steps {
					if strings.HasPrefix(str(get(other, "uses")), "docker/metadata-action") {
						for _, im := range splitList(expandGH(str(at(other, "with", "images")), combo, jobEnv)) {
							b.Names = append(b.Names, NormalizeImage(im))
						}
					}
				}
				if str(get(with, "push")) == "true" || strings.Contains(str(get(with, "push")), "${{") {
					s.Verb = "publish"
				} else {
					s.Verb = "package"
				}
				if b.Context != "" {
					p.addBuild(b)
				}
			case strings.HasPrefix(uses, "azure/k8s-deploy"):
				for _, m := range splitList(expandGH(str(get(with, "manifests")), combo, jobEnv)) {
					p.addDeploy(&deployHit{Kind: "kubectl", Target: joinPath(p.root, m), Line: ln})
				}
			case strings.HasPrefix(uses, "google-github-actions/deploy-cloudrun"):
				p.addDeploy(&deployHit{Kind: "cloud", Target: expandGH(str(get(with, "service"))+str(get(with, "job")), combo, jobEnv), Line: ln})
			case strings.HasPrefix(uses, "azure/webapps-deploy"), strings.HasPrefix(uses, "azure/functions-action"):
				p.addDeploy(&deployHit{Kind: "cloud", Target: expandGH(str(get(with, "app-name")), combo, jobEnv), Line: ln})
			case strings.HasPrefix(uses, "aws-actions/amazon-ecs-deploy-task-definition"):
				p.addDeploy(&deployHit{Kind: "cloud", Target: expandGH(str(get(with, "service")), combo, jobEnv), Line: ln})
			case strings.HasPrefix(uses, "azure/setup-helm"), strings.HasPrefix(uses, "azure/setup-kubectl"):
			}
		}
	}
}

func pinOfImage(ref string) string {
	switch {
	case strings.Contains(ref, "@sha256:"):
		return "sha"
	case strings.Contains(ref[strings.LastIndex(ref, "/")+1:], ":") && !strings.HasSuffix(ref, ":latest"):
		return "version"
	}
	return "none"
}

func lineCount(content []byte) int {
	n := 1
	for _, c := range content {
		if c == '\n' {
			n++
		}
	}
	return n
}
