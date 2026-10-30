package cicd

import (
	stdpath "path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Rancher, three ways it shows in a repository:
//
//   - CLI deploys inside any CI job: `rancher login <server> --context
//     <cluster>:<project>` then `rancher kubectl apply -f k8s/…`. Read by the
//     shell reader (classify.go); the environment comes from the job, then
//     from the per-environment variables the context is written with
//     ($RANCHER_PROJECT_PRD), then from the job's name.
//   - Fleet: each fleet.yaml is a bundle Fleet deploys from git, a pipeline
//     of its own (system rancher_fleet, kind gitops) with one job per target.
//     A GitRepo resource says which repository, branch and paths Fleet
//     watches.
//   - Rancher Pipelines (Rancher 2.0–2.4): .rancher-pipeline.yml, stages of
//     steps, read like any CI file.

var envSuffix = regexp.MustCompile(`(?i)(?:_|-|\b)(DEV|DEVELOP|TST|TEST|QA|UAT|ACC|ACCEPT|ACCEPTANCE|STG|STAGE|STAGING|PREPROD|PRD|PROD|PRODUCTION|LIVE|NP|NONPROD)\b`)

// envOfText reads an environment out of variable names and labels:
// $RANCHER_PROJECT_PRD is production, $RANCHER_SERVER_NP non-production.
func envOfText(s string) string {
	ms := envSuffix.FindAllStringSubmatch(s, -1)
	if len(ms) == 0 {
		return ""
	}
	// The most specific one: PRD beats P, a project beats a server.
	best := ""
	for _, m := range ms {
		v := strings.ToLower(m[1])
		switch v {
		case "np", "nonprod":
			if best == "" {
				best = "non-production"
			}
		default:
			return v
		}
	}
	return best
}

// ---------------------------------------------------------------------------
// Fleet
// ---------------------------------------------------------------------------

func readFleet(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "rancher_fleet")
	p.Kind = "gitops"
	dir := dirOf(file)
	p.Name = stdpath.Base(dir)
	if dir == "." {
		p.Name = "fleet"
	}
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Read = "none"
		p.unread(1, lineCount(content), "unsupported_syntax")
		return p
	}
	doc := docs[0]
	p.trigger("push", "watched by Fleet; the GitRepo is not in this workspace", 1)
	ns := str(get(doc, "defaultNamespace"))
	if ns == "" {
		ns = str(get(doc, "namespace"))
	}
	deploy := func(spec *yaml.Node, defaults *yaml.Node, env string) {
		helm := get(spec, "helm")
		if helm == nil {
			helm = get(defaults, "helm")
		}
		kust := get(spec, "kustomize")
		if kust == nil {
			kust = get(defaults, "kustomize")
		}
		switch {
		case helm != nil:
			chart := str(get(helm, "chart"))
			repo := str(get(helm, "repo"))
			version := str(get(helm, "version"))
			ln := line(helm)
			target := dir
			if chart != "" && repo == "" && !strings.Contains(chart, "://") {
				target = joinPath(dir, chart)
				h := p.handoff("helm_chart", target, "", ln)
				h.Pinned, h.Local, h.Job = "", target, ""
			} else if chart != "" {
				p.handoff("helm_chart", strings.TrimSuffix(repo, "/")+"/"+chart, version, ln).Job = ""
				target = chart
			}
			s := p.step("helm "+chart, "helm chart "+chart, ln)
			s.Verb, s.Tool, s.sure = "deploy", "helm", true
			p.addDeploy(&deployHit{Kind: "helm", Platform: "fleet", Target: target, Line: ln, Environment: env})
			for _, vf := range strs(get(helm, "valuesFiles")) {
				p.addDeploy(&deployHit{Kind: "helm_values", Platform: "fleet", Target: joinPath(dir, vf), Line: ln, Environment: env})
			}
		case kust != nil:
			kdir := joinPath(dir, str(get(kust, "dir")))
			s := p.step("kustomize "+str(get(kust, "dir")), "kustomize build "+kdir, line(kust))
			s.Verb, s.Tool, s.sure = "deploy", "kustomize", true
			p.addDeploy(&deployHit{Kind: "kustomize", Platform: "fleet", Target: kdir, Line: line(kust), Environment: env})
		default:
			s := p.step("manifests", "kubectl apply -f "+dir, 1)
			s.Verb, s.Tool, s.sure = "deploy", "kubectl", true
			p.addDeploy(&deployHit{Kind: "kubectl", Platform: "fleet", Target: dir, Line: 1, Environment: env})
		}
	}
	targets := items(get(doc, "targetCustomizations"))
	if len(targets) == 0 {
		j := p.job("deploy", "deploy", 1)
		j.Runner = "fleet"
		if ns != "" {
			j.Runner += " · " + ns
		}
		deploy(doc, nil, "")
	}
	for _, t := range targets {
		name := str(get(t, "name"))
		if name == "" {
			name = "target"
		}
		j := p.job(name, name, line(t))
		j.Runner = "fleet"
		if g := str(get(t, "clusterGroup")); g != "" {
			j.Runner += " · cluster group " + g
		} else if c := str(get(t, "clusterName")); c != "" {
			j.Runner += " · cluster " + c
		} else if sel := get(t, "clusterSelector"); sel != nil {
			var parts []string
			for _, kv := range pairs(get(sel, "matchLabels")) {
				parts = append(parts, kv.Key+"="+str(kv.Value))
			}
			if len(parts) > 0 {
				j.Runner += " · clusters " + strings.Join(parts, ",")
			}
		}
		env := envOfText(name + " " + j.Runner)
		if env == "" {
			env = name
		}
		j.Environment = env
		deploy(t, doc, env)
	}
	p.finish()
	return p
}

// A gitRepo is a Fleet GitRepo resource: which repository, branch and paths
// Fleet deploys, and to which clusters.
type gitRepo struct {
	Name, Repo, Branch, Revision string
	Paths                        []string
	Targets                      []string
	File                         string
	Line                         int
}

var fleetAPI = regexp.MustCompile(`(?m)^apiVersion:\s*fleet\.cattle\.io/`)

func readGitRepos(file string, content []byte) []*gitRepo {
	if !fleetAPI.Match(content) {
		return nil
	}
	var out []*gitRepo
	for _, doc := range yamlDocs(content) {
		if str(get(doc, "kind")) != "GitRepo" {
			continue
		}
		spec := get(doc, "spec")
		g := &gitRepo{Name: str(at(doc, "metadata", "name")), Repo: str(get(spec, "repo")), Branch: str(get(spec, "branch")),
			Revision: str(get(spec, "revision")), Paths: strs(get(spec, "paths")), File: file, Line: line(doc)}
		for _, t := range items(get(spec, "targets")) {
			name := str(get(t, "name"))
			if name == "" {
				name = str(get(t, "clusterGroup"))
			}
			if name != "" {
				g.Targets = append(g.Targets, name)
			}
		}
		out = append(out, g)
	}
	return out
}

// ---------------------------------------------------------------------------
// Rancher Pipelines (.rancher-pipeline.yml)
// ---------------------------------------------------------------------------

func readRancherPipeline(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "rancher_pipelines")
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Read = "none"
		p.unread(1, lineCount(content), "unsupported_syntax")
		return p
	}
	doc := docs[0]
	if br := get(doc, "branch"); br != nil {
		p.trigger("push", strings.Join(strs(get(br, "include")), ", "), line(br))
	} else {
		p.trigger("push", "", 1)
	}
	var prev []string
	for _, st := range items(get(doc, "stages")) {
		name := str(get(st, "name"))
		j := p.job(name, name, line(st))
		j.Needs = append(j.Needs, prev...)
		j.Condition = rancherWhen(get(st, "when"))
		for _, step := range items(get(st, "steps")) {
			ln := line(step)
			switch {
			case get(step, "runScriptConfig") != nil:
				c := get(step, "runScriptConfig")
				script := str(get(c, "shellScript"))
				if j.Runner == "" {
					j.Runner = str(get(c, "image"))
				}
				for _, l := range strings.Split(script, "\n") {
					if strings.TrimSpace(l) != "" {
						p.step("", l, ln)
					}
				}
				p.readShell(p.expand(script), ln, p.root)
			case get(step, "publishImageConfig") != nil:
				c := get(step, "publishImageConfig")
				tag := str(get(c, "tag"))
				s := p.step("publish image", "publish image "+tag, ln)
				s.Verb, s.Tool, s.sure = "publish", "docker", true
				ctx := str(get(c, "buildContext"))
				if ctx == "" {
					ctx = "."
				}
				df := str(get(c, "dockerfilePath"))
				if df == "" {
					df = stdpath.Join(ctx, "Dockerfile")
				}
				p.addBuild(&imageBuild{Names: []ImageName{NormalizeImage(tag)}, RawTags: []string{tag}, BuiltBy: "pipeline", File: file, Line: ln,
					Context: joinPath(p.root, ctx), Dockerfile: joinPath(p.root, df)})
				p.pushed = append(p.pushed, pushHit{Raw: tag, File: p.curFile, Line: ln, Job: j.ID})
			case get(step, "applyYamlConfig") != nil:
				c := get(step, "applyYamlConfig")
				path := str(get(c, "path"))
				s := p.step("apply yaml", "rancher apply "+path, ln)
				s.Verb, s.Tool, s.sure = "deploy", "rancher", true
				p.addDeploy(&deployHit{Kind: "rancher", Platform: "rancher", Target: joinPath(p.root, path), Line: ln})
			case get(step, "applyAppConfig") != nil:
				c := get(step, "applyAppConfig")
				tmpl := str(get(c, "catalogTemplate"))
				s := p.step("apply app", "rancher app "+tmpl+" "+str(get(c, "version")), ln)
				s.Verb, s.Tool, s.sure = "deploy", "rancher", true
				p.handoff("helm_chart", tmpl, str(get(c, "version")), ln)
				p.addDeploy(&deployHit{Kind: "rancher", Platform: "rancher", Target: str(get(c, "name")), Context: str(get(c, "targetNamespace")), Line: ln})
			default:
				p.unread(ln, ln, "unsupported_syntax")
			}
		}
		r := &jenkinsReader{p: p}
		r.envFromName(j)
		prev = []string{j.ID}
	}
	p.finish()
	return p
}

func rancherWhen(n *yaml.Node) string {
	var parts []string
	for _, kv := range pairs(n) {
		v := str(kv.Value)
		if v == "" {
			v = strings.Join(strs(kv.Value), ", ")
			if v == "" {
				v = strings.Join(strs(get(kv.Value, "include")), ", ")
			}
		}
		parts = append(parts, kv.Key+": "+v)
	}
	return strings.Join(parts, ", ")
}
