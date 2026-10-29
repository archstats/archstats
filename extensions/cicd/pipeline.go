package cicd

import (
	stdpath "path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A pipeline is read the way it runs: what starts it, the jobs it runs and
// what each waits for, the steps inside each job, what it hands work to, and
// what it produces. Every reader -- GitHub Actions, GitLab, Jenkins, Rancher,
// Azure, Bitbucket, CircleCI -- fills the same facts, so a job means the same
// thing whichever system wrote it:
//
//   - GitHub Actions: a job. GitLab: a job. Jenkins: a stage. Azure: a job.
//     Bitbucket: a step. CircleCI: a workflow job. Fleet: a target.
//   - A job's needs are the jobs it waits for. Systems that run in order
//     (GitLab stages, Jenkins stages, Bitbucket steps) write that order as
//     needs when they are read.
//
// What a reader cannot follow it says so, as a range of lines and a reason,
// instead of leaving a gap nobody can see.

// The verbs a step is read as, in the order a strip shows them.
var stageOrder = []string{"build", "test", "scan", "package", "publish", "deploy", "approve"}

type pipelineFact struct {
	ID     string
	Name   string
	System string // github_actions, gitlab, jenkins, rancher_fleet, rancher_pipelines, azure_pipelines, bitbucket, circleci, …
	// workflow, reusable_workflow, action (composite, docker, javascript),
	// library (a Jenkins shared library), gitops (a Fleet bundle), template
	// (a file only included by others) or pipeline.
	Kind string
	File string
	Line int
	Read string // full, partial, none

	Triggers []triggerFact
	Jobs     []*jobFact
	Handoffs []*handoffFact
	Unread   []unreadFact

	// Paths a trigger watches, for crediting a build to what it builds.
	Paths []string
	// Local workflows and actions this one uses, by path, and the line of
	// the first use.
	Calls     []string
	callLines map[string]int

	Builds   []*imageBuild
	Deploys  []*deployHit
	Runtimes []runtimeHit
	// Steps that run a build tool (mvn, gradle, npm, dotnet), which build
	// whatever module the repository holds.
	BuildTool     bool
	BuildToolLine int
	buildToolJob  string
	// The build runs in a template held outside this workspace.
	buildDelegated bool
	// A build tool runs in this file's own steps.
	buildLocal bool
	// Image pushes, for the latest-only flag.
	pushed []pushHit
	// The Rancher server and context each job last logged in to.
	rancherServer, rancherContext map[string]string

	// The local workflows and actions it uses, resolved: their images count
	// as built by this pipeline too.
	uses []*pipelineFact
	// What those build and deploy, credited to the job that uses them.
	folded        []*imageBuild
	foldedDeploys []*deployHit

	// Whether permissions are set for every job (GitHub Actions).
	permissions bool
	flags       []flagHit

	// References that name something the reader could not find.
	unresolved []unresolvedHit
	// The file being read when it is not the pipeline's own: an included
	// GitLab file, a Jenkins library step.
	curFile string

	cj *jobFact
	cs *stepFact
	// Variables known while reading: workflow env, GitLab variables. Used to
	// expand image names and Rancher targets, never written out.
	vars map[string]string
	// The folder its commands run in: paths in them are relative to it.
	root string
}

type flagHit struct {
	Rule, Detail string
	// The job it is about, when it is about one.
	Job  string
	Line int
}

// flag records a fact a hygiene rule reads as a risk.
func (p *pipelineFact) flag(rule, detail, job string, line int) {
	if job == "" && p.cj != nil {
		job = p.cj.ID
	}
	for _, f := range p.flags {
		if f.Rule == rule && f.Line == line && f.Detail == detail {
			return
		}
	}
	p.flags = append(p.flags, flagHit{Rule: rule, Detail: detail, Job: job, Line: line})
}

type unresolvedHit struct {
	Ref, Reason, File string
	Line              int
}

func (p *pipelineFact) unresolvedRef(ref string, line int, reason string) {
	f := p.curFile
	if f == "" {
		f = p.File
	}
	p.unresolved = append(p.unresolved, unresolvedHit{Ref: ref, Reason: reason, File: f, Line: line})
}

type triggerFact struct {
	Event  string // push, pull_request, tag, schedule, manual, called, upstream, other
	Filter string
	Line   int
}

type jobFact struct {
	ID, Name    string
	Group       string
	Needs       []string
	Runner      string
	Matrix      int
	Environment string
	Gate        string // manual, approval, environment
	Condition   string
	Line        int
	// The file the job is written in, when it is not the pipeline's own.
	File  string
	Steps []*stepFact
	Depth int
}

type stepFact struct {
	Index  int
	Name   string
	Verb   string
	Tool   string
	Target string
	Text   string
	Line   int
	File   string
	// Set when the verb came from what the step runs rather than its name,
	// so a later, better reading may replace a guess but not a fact.
	sure bool
}

type handoffFact struct {
	Job    string
	Kind   string // reusable_workflow, action, shared_library, library_step, load, include, component, template, orb, pipe, child_pipeline, starts_pipeline, helm_chart
	Target string
	Ref    string
	Pinned string // sha, version, branch, none, implicit
	Line   int
	// The pipeline it resolves to inside the workspace, when it does.
	Local string
	// For a library step: the library it comes from.
	Via string
}

type unreadFact struct {
	From, To int
	Reason   string // dynamic_stages, unknown_call, script_block, unsupported_syntax, not_supported_system
}

type deployHit struct {
	Kind     string // helm, helm_values, kubectl, kustomize, compose, skaffold, sam, cloud, rancher, fleet, delegated
	Platform string // rancher, kubernetes, helm, kustomize, fleet, compose, cloud, other
	Target   string
	// For Rancher: the server and the cluster:project context, as written.
	Server, Context string
	Environment     string
	// The image a step sets, when it names one (kubectl set image).
	Image ImageName
	Line  int
	Job   string
	Step  int
}

type runtimeHit struct {
	Tool, Version string
	Line          int
}

func newPipeline(file, system string) *pipelineFact {
	return &pipelineFact{ID: file, File: file, Line: 1, System: system, Kind: "pipeline", Name: stdpath.Base(file), Read: "full",
		callLines: map[string]int{}, vars: map[string]string{}, root: rootOf(file)}
}

// rootOf is the folder a CI file's commands run in: the repository root a
// .github, .circleci or .gitlab folder sits in, or the file's own folder.
func rootOf(file string) string {
	for _, marker := range []string{"/.github/", "/.circleci/", "/.gitlab/", "/.azure-pipelines/"} {
		if i := strings.Index("/"+file, marker); i >= 0 {
			if i == 0 {
				return "."
			}
			return ("/" + file)[1:i]
		}
	}
	return dirOf(file)
}

// job starts a job and makes it the one steps are added to. An id already
// used gets a number, so a Jenkinsfile with two stages called "Build" keeps
// both.
func (p *pipelineFact) job(id, name string, line int) *jobFact {
	if id == "" {
		id = name
	}
	if id == "" {
		id = "job"
	}
	base, n := id, 2
	for p.jobByID(id) != nil {
		id = base + " (" + strconv.Itoa(n) + ")"
		n++
	}
	if name == "" {
		name = id
	}
	j := &jobFact{ID: id, Name: name, Line: line, File: p.curFile}
	p.Jobs = append(p.Jobs, j)
	p.cj, p.cs = j, nil
	return j
}

func (p *pipelineFact) jobByID(id string) *jobFact {
	for _, j := range p.Jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// implicitJob is the job steps go into when a file has steps but no jobs:
// an action, a one-job Azure file, a Jenkinsfile with no stages.
func (p *pipelineFact) implicitJob(line int) *jobFact {
	if p.cj == nil {
		p.job(p.Name, p.Name, line)
	}
	return p.cj
}

// step adds a step to the current job and reads what it says. name is the
// step's label; text is what it runs or uses.
func (p *pipelineFact) step(name, text string, line int) *stepFact {
	j := p.implicitJob(line)
	s := &stepFact{Index: len(j.Steps) + 1, Name: oneLine(name, 120), Text: maskSecrets(oneLine(text, 160)), Line: line, File: p.curFile}
	j.Steps = append(j.Steps, s)
	p.cs = s
	verb, sure := classifyVerb(text)
	if verb == "" {
		verb, _ = classifyVerb(name)
	}
	if verb == "" {
		verb = "other"
	}
	s.Verb, s.sure = verb, sure
	s.Tool = classifyTool(text)
	if s.Tool == "" {
		s.Tool = classifyTool(name)
	}
	return s
}

func (p *pipelineFact) trigger(event, filter string, line int) {
	for _, t := range p.Triggers {
		if t.Event == event && t.Filter == filter {
			return
		}
	}
	p.Triggers = append(p.Triggers, triggerFact{Event: event, Filter: oneLine(filter, 200), Line: line})
}

func (p *pipelineFact) handoff(kind, target, ref string, line int) *handoffFact {
	h := &handoffFact{Kind: kind, Target: target, Ref: ref, Pinned: pinOf(ref), Line: line}
	if p.cj != nil {
		h.Job = p.cj.ID
	}
	p.Handoffs = append(p.Handoffs, h)
	return h
}

func (p *pipelineFact) unread(from, to int, reason string) {
	if to < from {
		to = from
	}
	for _, u := range p.Unread {
		if u.From <= from && u.To >= to {
			return
		}
	}
	p.Unread = append(p.Unread, unreadFact{From: from, To: to, Reason: reason})
	if p.Read == "full" {
		p.Read = "partial"
	}
}

// call records a local workflow or action this pipeline uses.
func (p *pipelineFact) call(target string, line int) {
	if _, ok := p.callLines[target]; !ok {
		p.Calls = append(p.Calls, target)
		p.callLines[target] = line
	}
}

// addBuild records an image build in the current job and step.
func (p *pipelineFact) addBuild(b *imageBuild) {
	if p.cj != nil {
		b.Job = p.cj.ID
	}
	if p.cs != nil {
		b.Step = p.cs.Index
		if p.cs.Target == "" && len(b.Names) > 0 {
			p.cs.Target = b.Names[0].String()
		}
	}
	p.Builds = append(p.Builds, b)
}

// addDeploy records a deploy in the current job and step.
func (p *pipelineFact) addDeploy(d *deployHit) {
	if p.cj != nil {
		d.Job = p.cj.ID
		if d.Environment == "" {
			d.Environment = p.cj.Environment
		}
	}
	if p.cs != nil {
		d.Step = p.cs.Index
		if p.cs.Target == "" {
			p.cs.Target = d.Target
		}
		if p.cs.Verb != "deploy" && p.cs.Verb != "approve" {
			p.cs.Verb = "deploy"
		}
	}
	if d.Platform == "" {
		d.Platform = platformOf(d.Kind)
	}
	if d.Environment == "" {
		d.Environment = envOfText(d.Context + " " + d.Server)
	}
	if d.Environment == "" && p.cj != nil {
		d.Environment = envOfText(p.cj.Name + " " + p.cj.Group)
	}
	p.Deploys = append(p.Deploys, d)
}

func platformOf(kind string) string {
	switch kind {
	case "helm", "helm_values":
		return "helm"
	case "kubectl":
		return "kubernetes"
	case "kustomize", "rancher", "fleet", "compose":
		return kind
	case "cloud", "sam":
		return "cloud"
	}
	return "other"
}

// finish computes what is derived once a file is read: each job's depth,
// and the order of everything written out.
func (p *pipelineFact) finish() {
	byID := map[string]*jobFact{}
	for _, j := range p.Jobs {
		byID[j.ID] = j
		// A need on a job that does not exist (a typo, or one an include
		// would have added) is kept as written, but lays out as nothing.
		sort.Strings(j.Needs)
		j.Needs = uniqueStrings(j.Needs)
	}
	state := map[string]int{} // 0 new, 1 visiting, 2 done
	var depth func(j *jobFact) int
	depth = func(j *jobFact) int {
		switch state[j.ID] {
		case 1:
			return 0 // a cycle: lay it out flat rather than not at all
		case 2:
			return j.Depth
		}
		state[j.ID] = 1
		d := 0
		for _, n := range j.Needs {
			if q := byID[n]; q != nil && q != j {
				if x := depth(q) + 1; x > d {
					d = x
				}
			}
		}
		j.Depth = d
		state[j.ID] = 2
		return d
	}
	for _, j := range p.Jobs {
		depth(j)
	}
	sort.SliceStable(p.Triggers, func(a, b int) bool { return p.Triggers[a].Line < p.Triggers[b].Line })
	sort.SliceStable(p.Unread, func(a, b int) bool { return p.Unread[a].From < p.Unread[b].From })
	seen := map[string]bool{}
	var hs []*handoffFact
	for _, h := range p.Handoffs {
		k := h.Job + "|" + h.Kind + "|" + h.Target + "@" + h.Ref
		if !seen[k] {
			seen[k] = true
			hs = append(hs, h)
		}
	}
	p.Handoffs = hs
	sort.Strings(p.Calls)
}

// Steps lists the verbs of every step, in strip order, once each.
func (p *pipelineFact) Stages() []string {
	have := map[string]bool{}
	for _, j := range p.Jobs {
		for _, s := range j.Steps {
			have[s.Verb] = true
		}
	}
	var out []string
	for _, s := range stageOrder {
		if have[s] {
			out = append(out, s)
		}
	}
	return out
}

func (p *pipelineFact) stepCount() int {
	n := 0
	for _, j := range p.Jobs {
		n += len(j.Steps)
	}
	return n
}

var (
	shaRef     = regexp.MustCompile(`^[0-9a-f]{40}$|^[0-9a-f]{64}$`)
	versionRef = regexp.MustCompile(`^v?\d+(\.\d+)*([-+.][0-9A-Za-z.-]+)?$|^~?\d`)
)

// pinOf says how firmly a reference names what it uses: a commit, a version,
// a branch, or nothing at all.
func pinOf(ref string) string {
	switch {
	case ref == "":
		return "none"
	case ref == "implicit":
		return "implicit"
	case shaRef.MatchString(ref):
		return "sha"
	case versionRef.MatchString(ref):
		return "version"
	}
	return "branch"
}

func oneLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i]) + " …"
	}
	if len(s) > max {
		cut := max
		for cut > 0 && !utf8Start(s[cut]) {
			cut--
		}
		s = s[:cut] + "…"
	}
	return s
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

var secretArg = regexp.MustCompile(`(?i)((?:--?(?:token|password|passwd|pass|secret|api-?key|access-?key|p)[ =])|(?:(?:password|passwd|token|secret|api_?key)\s*[=:]\s*))("[^"]*"|'[^']*'|\S+)`)

// maskSecrets replaces what follows a password or token flag with ***,
// unless it is a variable: `--token $KUBE_TOKEN` says which secret is used,
// and that is worth keeping.
func maskSecrets(s string) string {
	return secretArg.ReplaceAllStringFunc(s, func(m string) string {
		sub := secretArg.FindStringSubmatch(m)
		v := strings.Trim(sub[2], `"'`)
		if isVariable(v) {
			return m
		}
		return sub[1] + "***"
	})
}

var variableRef = regexp.MustCompile(`^\$\{?[A-Za-z_][A-Za-z0-9_.]*\}?$|^\$\{\{.*\}\}$|^%[A-Za-z_]+%$|^\$\([A-Za-z_.]+\)$|^env\.[A-Za-z_]+$|^credentials\(`)

func isVariable(v string) bool { return variableRef.MatchString(strings.TrimSpace(v)) }

func uniqueStrings(xs []string) []string {
	var out []string
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}
