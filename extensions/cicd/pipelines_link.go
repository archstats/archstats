package cicd

import (
	stdpath "path"
	"regexp"
	"sort"
	"strings"
)

// Pipelines meet the rest of the workspace here: what they call is resolved
// to other pipelines (in this repository or another one the workspace
// holds), what they build and deploy is joined to artifacts, and every fact
// is written out as rows.

func (b *builder) readPipeline(f, kind string, content []byte) *pipelineFact {
	rc := b.readCtx(f)
	if kind == "ci_library" {
		return readJenkinsLibrary(f, content, rc)
	}
	switch sys := ciSystemOf(f); sys {
	case "github_actions":
		return readGitHubWorkflow(f, content)
	case "github_action":
		return readGitHubAction(f, content)
	case "jenkins":
		return readJenkinsfile(f, content, rc)
	case "gitlab":
		return readGitLabCI(f, content, rc)
	case "azure_pipelines":
		return readAzurePipelines(f, content, rc)
	case "bitbucket":
		return readBitbucket(f, content)
	case "circleci":
		return readCircleCI(f, content)
	case "rancher_pipelines":
		return readRancherPipeline(f, content)
	default:
		// A workflow written somewhere else, as a template to copy.
		if workflowShape.Match(content) {
			p := readGitHubWorkflow(f, content)
			p.Kind = "template"
			p.copyOnly = true
			return p
		}
		return readOtherCI(f, sys)
	}
}

var workflowShape = regexp.MustCompile(`(?m)^jobs:\s*$`)

// repoDir is the folder of the repository a file is in, "." for the root.
func (b *builder) repoDir(f string) string {
	r := b.in.RepoOf(f)
	if r != "" && b.dirSet[r] && r != "." {
		return r
	}
	return "."
}

func (b *builder) readCtx(f string) *readCtx {
	return &readCtx{
		lookup: func(p string) ([]byte, bool) {
			c, ok := b.in.Files[p]
			return c, ok
		},
		glob: func(pattern string) []string {
			re := globRegexp(pattern)
			if re == nil {
				return nil
			}
			var out []string
			for _, f := range sortedKeys(b.in.Files) {
				if re.MatchString(f) {
					out = append(out, f)
				}
			}
			return out
		},
		repo:     b.repoDir(f),
		repoName: stdpath.Base(b.in.RepoOf(f)),
		project:  b.projectFile,
	}
}

// projectFile finds a file of another project the workspace holds: by the
// path of its origin remote, then by its folder's name. A tie is refused.
func (b *builder) projectFile(project, file string) (string, bool) {
	dir, ok := b.projectDir(project)
	if !ok {
		return "", false
	}
	p := joinPath(dir, strings.TrimPrefix(file, "/"))
	if _, ok := b.in.Files[p]; ok {
		return p, true
	}
	if b.allFileSet[p] {
		return p, true
	}
	return "", false
}

func (b *builder) projectDir(project string) (string, bool) {
	project = strings.ToLower(strings.Trim(project, "/"))
	if project == "" {
		return "", false
	}
	var byRemote, byName []string
	last := stdpath.Base(project)
	for _, dir := range sortedKeys(b.in.Remotes) {
		remote := strings.ToLower(b.in.Remotes[dir])
		if remote == project || strings.HasSuffix(remote, "/"+project) {
			byRemote = append(byRemote, dir)
		}
	}
	if len(byRemote) == 1 {
		return byRemote[0], true
	}
	if len(byRemote) > 1 {
		return "", false
	}
	for d := range b.dirSet {
		if strings.EqualFold(stdpath.Base(d), last) && (b.in.RepoOf(joinPath(d, "x")) == d) {
			byName = append(byName, d)
		}
	}
	if len(byName) == 1 {
		return byName[0], true
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Resolving pipelines to each other
// ---------------------------------------------------------------------------

func (b *builder) resolvePipelines() {
	byID := map[string]*pipelineFact{}
	for _, p := range b.pipelines {
		byID[p.ID] = p
	}
	find := func(target string) *pipelineFact {
		for _, id := range []string{target, joinPath(target, "action.yml"), joinPath(target, "action.yaml")} {
			if p := byID[id]; p != nil {
				return p
			}
		}
		return nil
	}
	// GitLab child pipelines and Azure templates name files no reader saw
	// as pipelines of their own: read them now.
	for i := 0; i < len(b.pipelines); i++ {
		p := b.pipelines[i]
		for _, h := range p.Handoffs {
			if h.Kind != "child_pipeline" || h.Local == "" || byID[h.Local] != nil {
				continue
			}
			if content, ok := b.in.Files[h.Local]; ok {
				c := readGitLabCI(h.Local, content, b.readCtx(h.Local))
				c.Kind = "child"
				b.pipelines = append(b.pipelines, c)
				byID[c.ID] = c
			}
		}
	}
	libraries := map[string][]*pipelineFact{}
	for _, p := range b.pipelines {
		if p.Kind == "library" {
			libraries[p.Name] = append(libraries[p.Name], p)
		}
	}
	for _, p := range b.pipelines {
		repo := b.repoDir(p.File)
		for _, h := range p.Handoffs {
			switch {
			case h.Local != "":
				if q := find(h.Local); q != nil {
					h.Local = q.ID
				}
			case p.System == "github_actions" && (h.Kind == "action" || h.Kind == "reusable_workflow") && h.Pinned == "" && !strings.Contains(h.Target, "@"):
				// A local action or workflow: ./.github/actions/setup
				if q := find(h.Target); q != nil {
					h.Local = q.ID
					p.uses = append(p.uses, q)
				} else if !strings.HasPrefix(h.Target, "docker://") {
					p.unresolvedRef(h.Target, h.Line, "not_found")
				}
			case p.System == "github_actions" && (h.Kind == "action" || h.Kind == "reusable_workflow"):
				// owner/repo/path: in another repository of the workspace?
				parts := strings.SplitN(h.Target, "/", 3)
				if len(parts) < 2 {
					continue
				}
				if dir, ok := b.projectDir(parts[0] + "/" + parts[1]); ok {
					rest := ""
					if len(parts) == 3 {
						rest = parts[2]
					}
					if q := find(joinPath(dir, rest)); q != nil {
						h.Local = q.ID
						p.uses = append(p.uses, q)
					}
				}
			case h.Kind == "library_step":
				cands := libraries[h.Target]
				if len(cands) > 1 && h.Via != "" {
					var narrowed []*pipelineFact
					for _, c := range cands {
						if strings.EqualFold(stdpath.Base(b.repoDir(c.File)), h.Via) {
							narrowed = append(narrowed, c)
						}
					}
					cands = narrowed
				}
				if len(cands) == 1 {
					h.Local = cands[0].ID
				}
			case h.Kind == "starts_pipeline" && p.System == "jenkins":
				// build job: 'folder/app/main' -- the job named after a
				// repository the workspace holds.
				var hits []*pipelineFact
				for _, seg := range strings.Split(h.Target, "/") {
					dir, ok := b.projectDir(seg)
					if !ok {
						continue
					}
					for _, q := range b.pipelines {
						if q.System == "jenkins" && q.Kind == "pipeline" && dirOf(q.File) == dir {
							hits = append(hits, q)
						}
					}
				}
				if len(hits) == 1 {
					h.Local = hits[0].ID
				}
			case h.Kind == "starts_pipeline" && p.System == "gitlab":
				if path, ok := b.projectFile(h.Target, ".gitlab-ci.yml"); ok && byID[path] != nil {
					h.Local = path
				}
			}
		}
		_ = repo
	}
	// workflow_run: the workflows named start this one.
	for _, p := range b.pipelines {
		if p.System != "github_actions" {
			continue
		}
		for _, t := range p.Triggers {
			if t.Event != "upstream" {
				continue
			}
			for _, part := range strings.Split(t.Filter, "; ") {
				names, ok := strings.CutPrefix(part, "workflows: ")
				if !ok {
					continue
				}
				for _, name := range strings.Split(names, ", ") {
					for _, q := range b.pipelines {
						if q != p && q.System == "github_actions" && q.Name == name && b.repoDir(q.File) == b.repoDir(p.File) {
							q.Handoffs = append(q.Handoffs, &handoffFact{Kind: "starts_pipeline", Target: p.File, Pinned: "", Line: 1, Local: p.ID})
						}
					}
				}
			}
		}
	}
	// Fleet: a GitRepo watching a repository the workspace holds names the
	// branch and the paths its bundles are deployed from.
	for _, g := range b.gitRepos {
		dir, ok := b.projectDir(repoPathOfURL(g.Repo))
		if !ok {
			continue
		}
		for _, p := range b.pipelines {
			if p.System != "rancher_fleet" || b.repoDir(p.File) != dir {
				continue
			}
			bundle := dirOf(p.File)
			watched := len(g.Paths) == 0
			for _, gp := range g.Paths {
				gp = joinPath(dir, strings.TrimPrefix(gp, "/"))
				if bundle == gp || strings.HasPrefix(bundle, gp+"/") {
					watched = true
				}
			}
			if !watched {
				continue
			}
			branch := firstNonEmpty(g.Branch, g.Revision, "default branch")
			p.Triggers = []triggerFact{{Event: "push", Filter: branch + " · GitRepo " + g.Name + " (" + g.File + ")", Line: 1}}
		}
	}
	// Folding: a local workflow or composite action runs inside the job
	// that uses it, so its images and deploys are the caller's too.
	for _, p := range b.pipelines {
		seen := map[*pipelineFact]bool{p: true}
		var walk func(q *pipelineFact, job string)
		walk = func(q *pipelineFact, job string) {
			for _, r := range q.uses {
				if seen[r] {
					continue
				}
				seen[r] = true
				for _, bd := range r.Builds {
					c := *bd
					c.Job, c.Step = job, 0
					p.folded = append(p.folded, &c)
				}
				for _, d := range r.Deploys {
					c := *d
					c.Job, c.Step, c.File = job, 0, firstNonEmpty(d.File, r.File)
					p.foldedDeploys = append(p.foldedDeploys, &c)
				}
				if r.BuildTool && !p.BuildTool {
					p.BuildTool, p.BuildToolLine, p.buildToolJob = true, 1, job
				}
				p.Runtimes = append(p.Runtimes, r.Runtimes...)
				walk(r, job)
			}
		}
		for _, h := range p.Handoffs {
			if h.Local == "" {
				continue
			}
			if q := byID[h.Local]; q != nil && (h.Kind == "action" || h.Kind == "reusable_workflow" || h.Kind == "library_step" || h.Kind == "load") {
				if !seen[q] || h.Kind == "library_step" {
					seen[q] = true
					for _, bd := range q.Builds {
						c := *bd
						c.Job, c.Step = h.Job, 0
						p.folded = append(p.folded, &c)
					}
					for _, d := range q.Deploys {
						c := *d
						c.Job, c.Step, c.File = h.Job, 0, firstNonEmpty(d.File, q.File)
						if c.Environment == "" {
							if j := p.jobByID(h.Job); j != nil {
								c.Environment = j.Environment
							}
						}
						p.foldedDeploys = append(p.foldedDeploys, &c)
					}
					if q.BuildTool && !p.BuildTool {
						p.BuildTool, p.BuildToolLine, p.buildToolJob = true, h.Line, h.Job
					}
					p.Runtimes = append(p.Runtimes, q.Runtimes...)
					walk(q, h.Job)
				}
			}
		}
	}
	sort.SliceStable(b.pipelines, func(i, j int) bool { return b.pipelines[i].ID < b.pipelines[j].ID })
}

var gitURLPath = regexp.MustCompile(`^(?:[a-z+]+://)?(?:[^@/]+@)?[^/:]+[:/](.+?)(?:\.git)?/?$`)

// repoPathOfURL reduces a git URL to its path: group/name.
func repoPathOfURL(u string) string {
	if m := gitURLPath.FindStringSubmatch(strings.TrimSpace(u)); m != nil {
		return m[1]
	}
	return u
}

// ---------------------------------------------------------------------------
// What pipelines produce
// ---------------------------------------------------------------------------

func (b *builder) outputs() {
	for _, p := range b.pipelines {
		repo := b.in.RepoOf(p.File)
		seen := map[string]bool{}
		add := func(o *OutputRow) {
			k := o.Job + "|" + itoa(o.Step) + "|" + o.Kind + "|" + o.Target + "|" + o.Artifact + "|" + o.Environment
			if seen[k] {
				return
			}
			seen[k] = true
			o.Pipeline = p.ID
			if o.File == "" {
				o.File = p.File
			}
			b.m.Outputs = append(b.m.Outputs, o)
		}
		builtSomething := false
		builds := append(append([]*imageBuild{}, p.Builds...), p.folded...)
		for _, bd := range builds {
			target := ""
			if len(bd.Names) > 0 {
				target = bd.Names[0].String()
			}
			if target == "" {
				target = bd.Dockerfile
			}
			if d := b.buildOf[bd]; d != nil {
				add(&OutputRow{Job: bd.Job, Step: bd.Step, Kind: "image", Target: firstNonEmpty(target, d.Name), Artifact: d.ID, Line: bd.Line, Resolution: "declared"})
				builtSomething = true
				continue
			}
			if (bd.BuiltBy == "jib" || bd.BuiltBy == "skaffold") && bd.Context == "" {
				for _, d := range b.m.Artifacts {
					if d.Repository != repo {
						continue
					}
					if bd.BuiltBy == "jib" && d.BuiltBy == "jib" || bd.BuiltBy == "skaffold" && d.priority == 1 {
						add(&OutputRow{Job: bd.Job, Step: bd.Step, Kind: "image", Target: d.Name, Artifact: d.ID, Line: bd.Line, Resolution: "repository"})
						builtSomething = true
					}
				}
				continue
			}
			add(&OutputRow{Job: bd.Job, Step: bd.Step, Kind: "image", Target: target, Line: bd.Line})
		}
		deployed := false
		for _, dh := range append(append([]*deployHit{}, p.Deploys...), p.foldedDeploys...) {
			ids := b.deployTargets(dh, repo)
			ctx := dh.Context
			platform := dh.Platform
			if ctx == "" {
				if proj := b.rancherProjectOf(dh.Target); proj != "" {
					ctx, platform = proj, "rancher"
				}
			}
			base := OutputRow{Job: dh.Job, Step: dh.Step, Kind: "deploy", Platform: platform, Environment: dh.Environment, Target: firstNonEmpty(dh.Target, dh.Kind), Server: dh.Server, Context: ctx, File: dh.File, Line: dh.Line}
			if len(ids) == 0 {
				add(&base)
				continue
			}
			deployed = true
			for _, id := range ids {
				o := base
				o.Artifact, o.Resolution = id, "path"
				add(&o)
			}
		}
		// Packages and releases published without an image.
		for _, j := range p.Jobs {
			for _, s := range j.Steps {
				if s.Verb != "publish" || s.Tool == "docker" {
					continue
				}
				kind := "package"
				if s.Tool == "gh" || strings.Contains(strings.ToLower(s.Text), "release") {
					kind = "release"
				}
				add(&OutputRow{Job: j.ID, Step: s.Index, Kind: kind, Target: firstNonEmpty(s.Target, s.Tool, s.Text), File: s.File, Line: s.Line})
			}
		}
		// A pipeline that builds without naming what: in a repository that
		// ships exactly one thing, it is that thing; in one with several,
		// what its path filter watches.
		action := p.Kind == "action" || p.Kind == "library" || p.Kind == "template"
		if p.BuildTool && !builtSomething && !action {
			var ids []string
			resolution := "repository"
			if only := b.onlyArtifactIn(repo); only != "" {
				ids = []string{only}
			} else if len(p.Paths) > 0 {
				ids = b.artifactsUnder(p.Paths, repo)
				resolution = "paths"
			}
			for _, id := range ids {
				add(&OutputRow{Job: p.buildToolJob, Kind: "build", Target: b.byID[id].Name, Artifact: id, Line: p.BuildToolLine, Resolution: resolution})
				// Built by a template held outside the workspace.
				if d := b.byID[id]; d != nil && d.Kind == "app" && p.buildDelegated && !p.buildLocal {
					d.BuiltBy = "delegated"
				}
			}
		}
		_ = deployed
		for _, u := range p.unresolved {
			b.m.Unresolved = append(b.m.Unresolved, &Unresolved{From: p.ID, Ref: u.Ref, File: u.File, Line: u.Line, Reason: u.Reason})
		}
	}
}

var cattleProject = regexp.MustCompile(`field\.cattle\.io/projectId:\s*['"]?([A-Za-z0-9:-]+)`)

// rancherProjectOf reads the Rancher project a manifest's namespace is
// annotated with.
func (b *builder) rancherProjectOf(target string) string {
	if target == "" {
		return ""
	}
	for _, f := range sortedKeys(b.in.Files) {
		if f != target && !strings.HasPrefix(f, target+"/") {
			continue
		}
		if m := cattleProject.FindSubmatch(b.in.Files[f]); m != nil {
			return string(m[1])
		}
	}
	return ""
}

func (b *builder) onlyArtifactIn(repo string) string {
	only := ""
	for _, d := range b.m.Artifacts {
		if d.Repository != repo {
			continue
		}
		if only != "" {
			return ""
		}
		only = d.ID
	}
	return only
}

// deployTargets resolves what a deploy step names to artifacts.
func (b *builder) deployTargets(dh *deployHit, repo string) []string {
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
				out = append(out, b.valuesOwner[v.File]...)
			}
		}
	case "kubectl", "kustomize", "rancher":
		for _, f := range b.expandKustomize(dh.Target) {
			for _, w := range b.workloads {
				if under(w.File, f) {
					out = append(out, b.runs[w.Key]...)
				}
			}
			for _, v := range b.values {
				if under(v.File, f) {
					out = append(out, b.valuesOwner[v.File]...)
				}
			}
		}
		if len(out) == 0 && (dh.Kind == "rancher" || dh.Kind == "kubectl") && dh.Target != "" && !strings.Contains(dh.Target, ".") {
			// deployment/api: the workload's name.
			name := normalizeName(dh.Target[strings.LastIndex(dh.Target, "/")+1:])
			for _, d := range b.m.Artifacts {
				if d.Name == name || containsStr(d.aliases, dh.Target) {
					out = append(out, d.ID)
				}
			}
		}
	case "skaffold":
		for _, d := range b.m.Artifacts {
			if d.Repository == repo && (d.BuiltBy == "skaffold" || d.BuiltBy == "jib" || d.BuiltBy == "buildpacks" || d.BuiltBy == "ko") {
				out = append(out, d.ID)
			}
		}
	case "compose":
		for _, d := range b.m.Artifacts {
			for _, bd := range d.builds {
				if bd.BuiltBy == "compose" && b.in.RepoOf(bd.File) == repo {
					out = append(out, d.ID)
				}
			}
		}
	case "sam":
		for _, d := range b.m.Artifacts {
			if d.Kind == "function" && d.Repository == repo {
				out = append(out, d.ID)
			}
		}
	case "cloud":
		name := normalizeName(dh.Target)
		for _, d := range b.m.Artifacts {
			if d.Name == name || containsStr(d.aliases, dh.Target) {
				out = append(out, d.ID)
			}
		}
	}
	if dh.Image.Repo != "" {
		if l := b.imageIndex().findImage(dh.Image); l.found() {
			out = append(out, l.ID)
		}
	}
	sort.Strings(out)
	var uniq []string
	for _, id := range out {
		uniq = appendUnique(uniq, id)
	}
	return uniq
}

func containsStr(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
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

// artifactsUnder is the artifacts holding a file some glob watches.
func (b *builder) artifactsUnder(globs []string, repo string) []string {
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
	for _, d := range b.m.Artifacts {
		if d.Repository != repo {
			continue
		}
	files:
		for _, c := range b.m.Contents {
			if c.Artifact != d.ID {
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

// globRegexp turns a path filter into a regular expression: `**` crosses
// folders, `*` does not.
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

// ---------------------------------------------------------------------------
// Rows
// ---------------------------------------------------------------------------

func (b *builder) pipelineRows() {
	m := b.m
	for _, p := range b.pipelines {
		m.Pipelines = append(m.Pipelines, &PipelineRow{ID: p.ID, Name: p.Name, System: p.System, Kind: p.Kind,
			Repository: b.in.RepoOf(p.File), File: p.File, Line: p.Line, Read: p.Read, Jobs: len(p.Jobs), Steps: p.stepCount(),
			Stages: strings.Join(p.Stages(), ",")})
		for _, u := range p.Unread {
			m.Unread = append(m.Unread, &UnreadRow{Pipeline: p.ID, FromLine: u.From, ToLine: u.To, Reason: u.Reason, File: p.File})
		}
		for _, t := range p.Triggers {
			m.Triggers = append(m.Triggers, &TriggerRow{Pipeline: p.ID, Event: t.Event, Filter: t.Filter, File: p.File, Line: t.Line})
		}
		for _, j := range p.Jobs {
			m.Jobs = append(m.Jobs, &JobRow{Pipeline: p.ID, ID: j.ID, Name: j.Name, Group: j.Group, Depth: j.Depth,
				Needs: strings.Join(j.Needs, ", "), Runner: oneLine(j.Runner, 200), Matrix: j.Matrix, Environment: j.Environment,
				Gate: j.Gate, Condition: j.Condition, File: firstNonEmpty(j.File, p.File), Line: j.Line})
			for _, s := range j.Steps {
				m.Steps = append(m.Steps, &StepRow{Pipeline: p.ID, Job: j.ID, Index: s.Index, Name: s.Name, Verb: s.Verb,
					Tool: s.Tool, Target: oneLine(s.Target, 200), Text: s.Text, File: firstNonEmpty(s.File, j.File, p.File), Line: s.Line})
			}
		}
		for _, h := range p.Handoffs {
			m.Handoffs = append(m.Handoffs, &HandoffRow{Pipeline: p.ID, Job: h.Job, Kind: h.Kind, Target: h.Target, Ref: h.Ref,
				Pinned: h.Pinned, Via: h.Via, LocalPipeline: h.Local, File: firstNonEmpty(h.File, p.File), Line: h.Line})
		}
	}
}

func (b *builder) dockerfileRows() {
	for _, f := range sortedKeys(b.dockerfiles) {
		df := b.dockerfiles[f]
		row := &DockerfileRow{File: f, Repository: b.in.RepoOf(f), Stages: len(df.Stages)}
		if n := len(df.Stages); n > 0 {
			last := df.Stages[n-1]
			row.FinalBase, _ = df.BaseImage(last)
			row.FinalUser = last.User
			row.Ports = strings.Join(last.Expose, ", ")
			row.Entrypoint = oneLine(last.Entrypoint, 200)
			if last.Healthcheck {
				row.Healthcheck = 1
			}
		}
		if b.hasDockerignore(f) {
			row.Dockerignore = 1
		}
		b.m.Dockerfiles = append(b.m.Dockerfiles, row)
		for _, st := range df.Stages {
			sr := &StageRow{File: f, Index: st.Index, Name: st.Name, From: st.From, User: st.User, Line: st.FromLine}
			switch {
			case strings.EqualFold(st.From, "scratch"):
				sr.FromKind = "scratch"
			case df.StageByRef(st.From, st.Index) != nil:
				sr.FromKind = "stage"
			default:
				sr.FromKind = "image"
				sr.Image, sr.Tag, sr.Digest = splitImageRef(st.From)
			}
			b.m.Stages = append(b.m.Stages, sr)
		}
	}
}

// hasDockerignore says whether a .dockerignore sits beside the Dockerfile
// or in a context a build uses with it.
func (b *builder) hasDockerignore(dockerfile string) bool {
	dirs := []string{dirOf(dockerfile)}
	for _, d := range b.m.Artifacts {
		for _, bd := range d.builds {
			// A Dockerfile's own build only guesses its folder.
			if bd.Dockerfile == dockerfile && bd.Context != "" && bd.BuiltBy != "dockerfile" {
				dirs = append(dirs, bd.Context)
			}
		}
	}
	if len(dirs) == 1 {
		// No build here says which folder is the context: it is the
		// Dockerfile's own or one above it, often the repository's root
		// (docker build -f docker/app/Dockerfile .).
		repo := b.repoDir(dockerfile)
		for d := dirOf(dockerfile); d != "" && d != "." && d != repo; {
			d = dirOf(d)
			dirs = append(dirs, d)
		}
	}
	base := stdpath.Base(dockerfile)
	for _, d := range dirs {
		if b.allFileSet[joinPath(d, ".dockerignore")] || b.allFileSet[joinPath(d, base+".dockerignore")] {
			return true
		}
		if _, ok := b.in.Files[joinPath(d, ".dockerignore")]; ok {
			return true
		}
	}
	return false
}

// splitImageRef splits registry/name:tag@digest into its parts.
func splitImageRef(ref string) (name, tag, digest string) {
	name = ref
	if i := strings.Index(name, "@"); i >= 0 {
		name, digest = name[:i], name[i+1:]
	}
	if slash, colon := strings.LastIndex(name, "/"), strings.LastIndex(name, ":"); colon > slash {
		name, tag = name[:colon], name[colon+1:]
	}
	return name, tag, digest
}

func (b *builder) sortPipelineRows() {
	m := b.m
	sort.SliceStable(m.Pipelines, func(i, j int) bool { return m.Pipelines[i].ID < m.Pipelines[j].ID })
	sort.SliceStable(m.Outputs, func(i, j int) bool {
		a, c := m.Outputs[i], m.Outputs[j]
		if a.Pipeline != c.Pipeline {
			return a.Pipeline < c.Pipeline
		}
		if a.Line != c.Line {
			return a.Line < c.Line
		}
		if a.Kind != c.Kind {
			return a.Kind < c.Kind
		}
		if a.Target != c.Target {
			return a.Target < c.Target
		}
		return a.Artifact < c.Artifact
	})
	sort.SliceStable(m.Flags, func(i, j int) bool {
		a, c := m.Flags[i], m.Flags[j]
		if a.Rule != c.Rule {
			return a.Rule < c.Rule
		}
		if a.File != c.File {
			return a.File < c.File
		}
		if a.Line != c.Line {
			return a.Line < c.Line
		}
		return a.Detail < c.Detail
	})
}
