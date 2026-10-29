package cicd

import (
	"regexp"
	"strings"
)

// Jenkins, declarative and scripted, read from the call tree groovy.go
// builds. A stage is a job; stages run in the order written, so each needs
// the one before it, and the branches of a parallel need what came before
// the parallel. What a Jenkinsfile calls that Jenkins does not define comes
// from a shared library: it is a hand-off to that library, named, with the
// ref the library is loaded at -- or "implicit" when no @Library line names
// it, because the Jenkins server loads it for every job.

// Jenkins steps and wrappers that are not library calls.
var jenkinsBuiltins = map[string]bool{
	"sh": true, "bat": true, "powershell": true, "pwsh": true, "echo": true, "println": true, "print": true,
	"checkout": true, "git": true, "script": true, "dir": true, "withCredentials": true, "withEnv": true,
	"timeout": true, "retry": true, "container": true, "node": true, "stage": true, "parallel": true,
	"input": true, "build": true, "junit": true, "archiveArtifacts": true, "stash": true, "unstash": true,
	"sleep": true, "error": true, "readFile": true, "writeFile": true, "readJSON": true, "readYaml": true,
	"writeYaml": true, "writeJSON": true, "readProperties": true, "readMavenPom": true, "fileExists": true, "cleanWs": true, "deleteDir": true,
	"emailext": true, "mail": true, "slackSend": true, "publishHTML": true, "jacoco": true, "recordIssues": true,
	"waitForQualityGate": true, "withSonarQubeEnv": true, "withMaven": true, "withDockerRegistry": true,
	"withKubeConfig": true, "sshagent": true, "configFileProvider": true, "lock": true, "milestone": true,
	"catchError": true, "warnError": true, "unarchive": true, "copyArtifacts": true, "httpRequest": true,
	"tool": true, "properties": true, "load": true, "library": true, "podTemplate": true, "containerTemplate": true,
	"wrap": true, "ansiColor": true, "timestamps": true, "buildDiscarder": true, "logRotator": true, "cron": true, "pollSCM": true,
	"upstream": true, "parameters": true, "when": true, "steps": true, "post": true, "always": true,
	"success": true, "failure": true, "unstable": true, "changed": true, "fixed": true, "regression": true,
	"aborted": true, "cleanup": true, "agent": true, "options": true, "environment": true, "tools": true,
	"triggers": true, "matrix": true, "axes": true, "axis": true, "excludes": true, "values": true, "label": true,
	"skipDefaultCheckout": true, "disableConcurrentBuilds": true, "pipeline": true, "stages": true,
	"credentials": true, "string": true, "booleanParam": true, "choice": true, "text": true, "password": true,
	"usernamePassword": true, "file": true, "sshUserPrivateKey": true, "usernameColonPassword": true,
	"pipelineTriggers": true, "githubPush": true, "gitlab": true, "gitLabConnection": true, "durabilityHint": true,
	"office365ConnectorSend": true, "notifyBitbucket": true, "setBuildStatus": true, "updateGitlabCommitStatus": true,
	"gitlabCommitStatus": true, "addBadge": true, "isUnix": true, "pwd": true, "findFiles": true, "zip": true,
	"unzip": true, "touch": true, "sha1": true, "sha256": true, "readCSV": true, "tee": true, "jiraComment": true,
	"withAWS": true, "s3Upload": true, "s3Download": true, "withGradle": true, "withNPM": true, "nodejs": true,
	"withPythonEnv": true, "kubernetes": true, "docker": true, "dockerfile": true, "any": true, "none": true,
	"allOf": true, "anyOf": true, "not": true, "branch": true, "tag": true, "expression": true, "changeRequest": true,
	"buildingTag": true, "changeset": true, "triggeredBy": true, "equals": true, "beforeAgent": true,
	"beforeInput": true, "beforeOptions": true, "skipStagesAfterUnstable": true, "quietPeriod": true,
	"checkoutToSubdirectory": true, "newContainerPerStage": true, "preserveStashes": true, "parallelsAlwaysFailFast": true,
	"overrideIndexTriggers": true, "rateLimitBuilds": true, "disableResume": true, "copyArtifactPermission": true,
	"githubProjectProperty": true, "throttleJobProperty": true, "addInfoBadge": true, "addErrorBadge": true,
	"addWarningBadge": true, "currentBuild": true, "env": true, "params": true, "scm": true, "def": true,
	"return": true, "fail": true, "assert": true, "unstable_": true, "Jenkins": true, "System": true, "println_": true,
	"emailextrecipients": true, "culprits": true, "developers": true, "requestor": true, "upstreamDevelopers": true,
	"lastDuration": true, "cobertura": true, "publishCoverage": true, "recordCoverage": true, "dependencyCheckPublisher": true,
	"step": true, "fingerprint": true, "nexusArtifactUploader": true, "nexusPublisher": true, "rtUpload": true,
	"rtServer": true, "rtMavenRun": true, "rtGradleRun": true, "rtPublishBuildInfo": true, "kubernetesDeploy": true,
	"ansiblePlaybook": true, "terraform": true, "dockerNode": true, "googleStorageUpload": true, "jiraSendBuildInfo": true,
	"jiraSendDeploymentInfo": true, "validateDeclarativePipeline": true, "unstash_": true, "buildName": true,
	"buildDescription": true, "currentBuildResult": true, "authorizationMatrix": true, "disableRestartFromStage": true,
	"getContext": true, "setContext": true, "parallelStages": true, "sendNotifications": false,
}

// Wrapper steps whose closure runs inside them.
var jenkinsWrappers = map[string]bool{
	"node": true, "podTemplate": true, "container": true, "dir": true, "withCredentials": true, "withEnv": true,
	"timeout": true, "retry": true, "timestamps": true, "ansiColor": true, "wrap": true, "lock": true,
	"catchError": true, "warnError": true, "withMaven": true, "withDockerRegistry": true, "withKubeConfig": true,
	"sshagent": true, "configFileProvider": true, "withSonarQubeEnv": true, "script": true, "steps": true,
	"withAWS": true, "withGradle": true, "withNPM": true, "nodejs": true, "withPythonEnv": true, "dockerNode": true,
	"always": true, "success": true, "failure": true, "unstable": true, "changed": true, "fixed": true,
	"regression": true, "aborted": true, "cleanup": true,
}

// Calls that say nothing about what the pipeline does.
var jenkinsQuiet = map[string]bool{
	"echo": true, "println": true, "print": true, "sleep": true, "error": true, "readFile": true, "writeFile": true,
	"readJSON": true, "readYaml": true, "writeYaml": true, "writeJSON": true, "readProperties": true, "readMavenPom": true,
	"fileExists": true, "properties": true, "isUnix": true, "pwd": true, "findFiles": true, "touch": true,
	"addBadge": true, "addInfoBadge": true, "addErrorBadge": true, "addWarningBadge": true, "buildName": true,
	"buildDescription": true, "milestone": true, "assert": true, "def": true, "return": true,
}

var jenkinsEnvWord = regexp.MustCompile(`(?i)\b(dev|develop|development|test|tst|qa|uat|acc|acceptance|stg|stage|staging|preprod|pre-prod|prod|production|prd|live|nonprod|np)\b`)

type jenkinsLib struct {
	Name, Ref string
	Line      int
}

type jenkinsReader struct {
	p    *pipelineFact
	defs map[string]*gNode
	libs []jenkinsLib
	// Branches a parallel will run, filled in by assignments the way
	// scripted pipelines build them: branches["x"] = { … }.
	branches map[string][]branch
	// Defined functions being inlined, to stop at recursion.
	inlining map[string]bool
	runner   string
	declared bool
	rc       *readCtx
	// Scripts already loaded, to stop at a loop.
	loaded map[string]bool
	// Names the file defines; a call on any other dotted name is a library's.
	locals map[string]bool
}

type branch struct {
	key   string
	block []*gNode
	line  int
	// Built inside a loop: one per item, how many is not in the file.
	repeat string
}

// jctx is where in the pipeline a statement sits.
type jctx struct {
	group  string
	prev   []string // the jobs whatever comes next waits for
	dir    string
	runner string
	repeat string // the collection a loop repeats this over
	cond   string
}

func readJenkinsfile(file string, content []byte, rc *readCtx) *pipelineFact {
	p := newPipeline(file, "jenkins")
	r := newJenkinsReader(p, rc)
	nodes := parseGroovy(string(content))
	r.locals = localNames(string(content))
	r.collect(nodes)
	r.read(nodes, &jctx{dir: dirOf(file)})
	if len(p.Triggers) == 0 {
		p.trigger("other", "set in the Jenkins job, not in this file", 1)
	}
	p.finish()
	return p
}

// readJenkinsLibrary reads a shared library's global step, vars/name.groovy:
// its call method is the pipeline every Jenkinsfile that calls name runs.
func readJenkinsLibrary(file string, content []byte, rc *readCtx) *pipelineFact {
	p := newPipeline(file, "jenkins")
	p.Kind = "library"
	base := file[strings.LastIndex(file, "/")+1:]
	p.Name = strings.TrimSuffix(base, ".groovy")
	p.trigger("called", "by Jenkinsfiles that call "+p.Name, 1)
	r := newJenkinsReader(p, rc)
	nodes := parseGroovy(string(content))
	r.locals = localNames(string(content))
	r.collect(nodes)
	if call := r.defs["call"]; call != nil {
		r.read(call.Block, &jctx{dir: "."})
	}
	p.finish()
	return p
}

func newJenkinsReader(p *pipelineFact, rc *readCtx) *jenkinsReader {
	if rc == nil {
		rc = &readCtx{}
	}
	return &jenkinsReader{p: p, rc: rc, defs: map[string]*gNode{}, branches: map[string][]branch{}, inlining: map[string]bool{}, loaded: map[string]bool{}, locals: map[string]bool{}}
}

// read reads statements as a declarative pipeline when they hold one, and
// as a scripted one when they do not.
func (r *jenkinsReader) read(nodes []*gNode, ctx *jctx) {
	for _, n := range nodes {
		if n.Kind == "call" && n.Name == "pipeline" && n.Block != nil {
			r.declared = true
			r.declarative(n, ctx)
			return
		}
	}
	r.scripted(nodes, ctx)
}

// collect finds the file's functions and the libraries it loads.
func (r *jenkinsReader) collect(nodes []*gNode) {
	for _, n := range nodes {
		if n.Kind == "def" {
			r.defs[n.Name] = n
		}
	}
	walkGroovy(nodes, func(n *gNode, _ []*gNode) {
		switch {
		case n.Kind == "call" && n.Name == "@Library":
			for _, a := range n.Args {
				for _, s := range libraryStrings(a) {
					r.addLib(s, n.Line)
				}
			}
		case n.Kind == "call" && (n.Name == "library" || n.Name == "lib"):
			if a := argOf(n.Args, "identifier", 0); a != nil {
				r.addLib(a.text(), n.Line)
			} else if a := argOf(n.Args, "", 0); a != nil {
				r.addLib(a.text(), n.Line)
			}
		case n.Kind == "assign":
			// def lib = library('x@y'): the library loads all the same.
			if m := libraryCall.FindStringSubmatch(n.Raw); m != nil {
				r.addLib(m[1], n.Line)
			}
		}
	})
}

var libraryCall = regexp.MustCompile(`\blibrary\s*\(?\s*(?:identifier\s*:\s*)?['"]([^'"]+)['"]`)

func libraryStrings(a *gArg) []string {
	if a.IsStr {
		return []string{a.Str}
	}
	var out []string
	for _, it := range a.Items {
		out = append(out, libraryStrings(it)...)
	}
	return out
}

func (r *jenkinsReader) addLib(spec string, line int) {
	name, ref, _ := strings.Cut(spec, "@")
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	for _, l := range r.libs {
		if l.Name == name {
			return
		}
	}
	r.libs = append(r.libs, jenkinsLib{Name: name, Ref: ref, Line: line})
	h := &handoffFact{Kind: "shared_library", Target: name, Ref: ref, Pinned: pinOf(ref), Line: line}
	r.p.Handoffs = append(r.p.Handoffs, h)
}

// libraryOf says which library a global step comes from: the one library
// the file loads, or implicit when it loads none. With several, the step
// could come from any of them.
func (r *jenkinsReader) libraryOf() (via, ref string) {
	switch len(r.libs) {
	case 0:
		return "implicit", "implicit"
	case 1:
		return r.libs[0].Name, r.libs[0].Ref
	}
	var names []string
	for _, l := range r.libs {
		names = append(names, l.Name)
	}
	return strings.Join(names, " | "), ""
}

// ---------------------------------------------------------------------------
// Declarative
// ---------------------------------------------------------------------------

func (r *jenkinsReader) declarative(pipeline *gNode, ctx *jctx) {
	p := r.p
	for _, n := range pipeline.Block {
		switch n.Name {
		case "agent":
			ctx.runner = r.agentOf(n)
			r.runner = ctx.runner
		case "triggers":
			for _, t := range n.Block {
				switch t.Name {
				case "cron":
					p.trigger("schedule", argOf(t.Args, "", 0).text(), t.Line)
				case "pollSCM":
					p.trigger("push", "poll SCM "+argOf(t.Args, "", 0).text(), t.Line)
				case "upstream":
					a := argOf(t.Args, "upstreamProjects", 0)
					p.trigger("upstream", a.text(), t.Line)
				case "githubPush", "gitlab", "bitbucketPush":
					p.trigger("push", t.Name, t.Line)
				default:
					p.trigger("other", t.Name, t.Line)
				}
			}
		case "environment":
			r.environment(n.Block)
		case "libraries":
			for _, l := range n.Block {
				if a := argOf(l.Args, "", 0); a != nil {
					r.addLib(a.text(), l.Line)
				}
			}
		case "parameters":
			if len(n.Block) > 0 {
				p.trigger("manual", "with parameters", n.Line)
			}
		}
	}
	for _, n := range pipeline.Block {
		if n.Name == "stages" {
			ctx.prev = r.declStages(n.Block, ctx)
		}
	}
	for _, n := range pipeline.Block {
		if n.Name == "post" && hasSteps(n.Block) {
			j := p.job("post", "post", n.Line)
			j.Needs = append(j.Needs, ctx.prev...)
			j.Runner = ctx.runner
			for _, cond := range n.Block {
				r.steps(cond.Block, &jctx{dir: ctx.dir, runner: ctx.runner}, cond.Name)
			}
		}
	}
}

func hasSteps(nodes []*gNode) bool {
	found := false
	walkGroovy(nodes, func(n *gNode, _ []*gNode) {
		if n.Kind == "call" && !jenkinsWrappers[n.Name] && !jenkinsQuiet[n.Name] {
			found = true
		}
	})
	return found
}

func (r *jenkinsReader) environment(nodes []*gNode) {
	for _, e := range nodes {
		if e.Kind == "assign" && !strings.Contains(e.Raw, "credentials(") {
			v := strings.Trim(e.Raw, `"'`)
			r.p.vars[e.Name] = v
		}
	}
}

// declStages reads a stages block; it returns the jobs that end it.
func (r *jenkinsReader) declStages(nodes []*gNode, ctx *jctx) []string {
	prev := ctx.prev
	for _, st := range nodes {
		if st.Name != "stage" {
			continue
		}
		name := argOf(st.Args, "", 0).text()
		inner := map[string]*gNode{}
		for _, c := range st.Block {
			if c.Kind == "call" {
				inner[c.Name] = c
			}
		}
		sub := *ctx
		sub.prev = prev
		if a := inner["agent"]; a != nil {
			sub.runner = r.agentOf(a)
		}
		if w := inner["when"]; w != nil {
			sub.cond = whenText(w.Block)
		}
		if e := inner["environment"]; e != nil {
			r.environment(e.Block)
		}
		switch {
		case inner["parallel"] != nil:
			sub.group = joinGroup(ctx.group, name)
			var ends []string
			for _, b := range inner["parallel"].Block {
				if b.Name != "stage" {
					continue
				}
				bctx := sub
				ends = append(ends, r.declStages([]*gNode{b}, &bctx)...)
			}
			prev = ends
		case inner["matrix"] != nil:
			m := inner["matrix"]
			size := 1
			var stagesNode *gNode
			for _, c := range m.Block {
				switch c.Name {
				case "axes":
					for _, ax := range c.Block {
						for _, v := range ax.Block {
							if v.Name == "values" {
								size *= max(len(v.Args), 1)
							}
						}
					}
				case "stages":
					stagesNode = c
				case "agent":
					sub.runner = r.agentOf(c)
				}
			}
			sub.group = joinGroup(ctx.group, name)
			sub.repeat = "matrix"
			if stagesNode != nil {
				before := len(r.p.Jobs)
				prev = r.declStages(stagesNode.Block, &sub)
				for _, j := range r.p.Jobs[before:] {
					j.Matrix = size
				}
			}
		case inner["stages"] != nil:
			sub.group = joinGroup(ctx.group, name)
			prev = r.declStages(inner["stages"].Block, &sub)
		default:
			j := r.p.job(name, name, st.Line)
			j.Group = ctx.group
			j.Needs = append(j.Needs, prev...)
			j.Runner = sub.runner
			j.Condition = sub.cond
			if ctx.repeat != "" && ctx.repeat != "matrix" {
				j.Matrix = -1
			}
			if in := inner["input"]; in != nil {
				j.Gate = "approval"
				s := r.p.step("input", "input "+argOf(in.Args, "message", 0).text(), in.Line)
				s.Verb, s.sure = "approve", true
			}
			if s := inner["steps"]; s != nil {
				r.steps(s.Block, &sub, "")
			}
			if post := inner["post"]; post != nil {
				for _, cond := range post.Block {
					r.steps(cond.Block, &sub, cond.Name)
				}
			}
			r.envFromName(j)
			prev = []string{j.ID}
		}
	}
	return prev
}

func joinGroup(a, b string) string {
	if a == "" {
		return b
	}
	return a + " › " + b
}

// whenText writes a when block as one line: branch main, tag v*, …
func whenText(nodes []*gNode) string {
	var parts []string
	for _, n := range nodes {
		switch n.Name {
		case "allOf", "anyOf", "not":
			parts = append(parts, n.Name+"("+whenText(n.Block)+")")
		case "expression":
			parts = append(parts, "expression")
		case "beforeAgent", "beforeInput", "beforeOptions":
		default:
			t := n.Name
			for _, a := range n.Args {
				if a.Key != "" {
					t += " " + a.Key + ": " + a.text()
				} else {
					t += " " + a.text()
				}
			}
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ", ")
}

func (r *jenkinsReader) agentOf(n *gNode) string {
	if a := argOf(n.Args, "", 0); a != nil && len(n.Block) == 0 {
		return a.text()
	}
	for _, c := range n.Block {
		switch c.Name {
		case "label":
			return argOf(c.Args, "", 0).text()
		case "docker":
			if a := argOf(c.Args, "", 0); a != nil {
				return "docker " + a.text()
			}
			for _, d := range c.Block {
				if d.Name == "image" {
					return "docker " + argOf(d.Args, "", 0).text()
				}
			}
			return "docker"
		case "dockerfile":
			return "dockerfile"
		case "kubernetes":
			for _, k := range c.Block {
				switch k.Name {
				case "yaml":
					if img := podImages(argOf(k.Args, "", 0).text()); img != "" {
						return "kubernetes · " + img
					}
				case "yamlFile":
					return "kubernetes · " + argOf(k.Args, "", 0).text()
				case "label", "inheritFrom", "cloud":
					return "kubernetes " + argOf(k.Args, "", 0).text()
				}
			}
			return "kubernetes"
		case "node":
			for _, d := range c.Block {
				if d.Name == "label" {
					return argOf(d.Args, "", 0).text()
				}
			}
		case "any", "none":
			return c.Name
		}
	}
	if len(n.Args) > 0 {
		return n.Args[0].text()
	}
	return ""
}

var podImage = regexp.MustCompile(`(?m)^\s*-?\s*image:\s*['"]?([^\s'"]+)`)

func podImages(yaml string) string {
	var out []string
	for _, m := range podImage.FindAllStringSubmatch(yaml, 6) {
		out = appendUnique(out, m[1])
	}
	return strings.Join(out, ", ")
}

// ---------------------------------------------------------------------------
// Scripted
// ---------------------------------------------------------------------------

// scripted reads a pipeline written as Groovy statements: node, stage and
// parallel calls among the rest. Steps outside every stage go into a job of
// their own.
func (r *jenkinsReader) scripted(nodes []*gNode, ctx *jctx) {
	r.block(nodes, ctx)
}

// block reads statements in order, carrying where the pipeline is.
func (r *jenkinsReader) block(nodes []*gNode, ctx *jctx) {
	for _, n := range nodes {
		r.statement(n, ctx)
	}
}

func (r *jenkinsReader) statement(n *gNode, ctx *jctx) {
	p := r.p
	switch n.Kind {
	case "def":
		return
	case "assign":
		if strings.HasPrefix(n.Name, "env.") {
			p.vars[strings.TrimPrefix(n.Name, "env.")] = strings.Trim(n.Raw, `"'`)
		} else if !strings.Contains(n.Name, "[") {
			p.vars[n.Name] = strings.Trim(n.Raw, `"'`)
		}
		// branches["x"] = { … }: a parallel branch, built for later.
		if i := strings.Index(n.Name, "["); i > 0 && len(n.Args) == 1 && n.Args[0].Block != nil {
			key := strings.Trim(n.Name[i+1:len(n.Name)-1], `"' `)
			r.branches[n.Name[:i]] = append(r.branches[n.Name[:i]], branch{key: key, block: n.Args[0].Block, line: n.Line, repeat: ctx.repeat})
			return
		}
		// def branches = [a: { … }, b: { … }]
		if len(n.Args) > 0 && n.Args[0].Block == nil {
			for _, it := range n.Args {
				if it.Key != "" && it.Block != nil {
					r.branches[n.Name] = append(r.branches[n.Name], branch{key: it.Key, block: it.Block, line: it.Line, repeat: ctx.repeat})
				}
			}
		}
		if m := dockerBuildDSL.FindStringSubmatch(n.Raw); m != nil {
			r.dockerBuild(m, n.Line, ctx)
		}
		return
	case "loop":
		sub := *ctx
		over := n.Over
		if over == "" {
			over = n.Name
		}
		sub.repeat = strings.TrimSpace(over)
		if containsStage(n.Block) || containsParallelBranch(n.Block) {
			r.block(n.Block, &sub)
		} else {
			r.block(n.Block, ctx)
		}
		ctx.prev = sub.prev
		if !containsStage(n.Block) && !containsParallelBranch(n.Block) {
			return
		}
		return
	case "if", "try", "other":
		if n.Kind == "if" && n.Over != "" {
			sub := *ctx
			sub.cond = strings.TrimSpace(strings.TrimPrefix(n.Over, "("))
			r.block(n.Block, &sub)
			ctx.prev = sub.prev
			return
		}
		r.block(n.Block, ctx)
		return
	}
	if n.Kind != "call" {
		return
	}
	if len(n.Chain) == 0 {
		n.Chain = []string{n.Name}
	}
	root := n.Chain[0]
	last := n.Chain[len(n.Chain)-1]
	switch {
	case n.Name == "@Library" || n.Name == "library" || n.Name == "lib":
		return
	case len(n.Chain) > 1 && last == "put" && len(n.Args) >= 2 && n.Args[len(n.Args)-1].Block != nil:
		// branches.put(name, { … }): a parallel branch, built for later.
		mapName := strings.Join(n.Chain[:len(n.Chain)-1], ".")
		r.branches[mapName] = append(r.branches[mapName], branch{key: n.Args[0].text(), block: n.Args[len(n.Args)-1].Block, line: n.Line, repeat: ctx.repeat})
		return
	case n.Name == "stage":
		r.stage(n, ctx)
	case n.Name == "parallel":
		r.parallel(n, ctx)
	case n.Name == "node" || n.Name == "podTemplate" || n.Name == "dockerNode":
		sub := *ctx
		label := argOf(n.Args, "label", 0).text()
		if n.Name == "podTemplate" {
			if y := argOf(n.Args, "yaml", -1); y != nil {
				label = "kubernetes · " + podImages(y.text())
			} else {
				label = "kubernetes " + label
			}
			var imgs []string
			if c := argOf(n.Args, "containers", -1); c != nil {
				walkContainers(c, &imgs)
			}
			if len(imgs) > 0 {
				label = "kubernetes · " + strings.Join(imgs, ", ")
			}
		}
		if label == "" {
			label = n.Name
		}
		if sub.runner == "" || n.Name == "node" && !strings.HasPrefix(sub.runner, "kubernetes") {
			sub.runner = label
		}
		r.block(n.Block, &sub)
		ctx.prev = sub.prev
	case n.Name == "dir":
		sub := *ctx
		sub.dir = joinPath(ctx.dir, argOf(n.Args, "", 0).text())
		r.block(n.Block, &sub)
		ctx.prev = sub.prev
	case r.defs[n.Name] != nil:
		// A function this file defines: read its body where it is called.
		// When it is passed closures (tryFinally({ … }, { … })), what runs
		// is those closures; the body only arranges them.
		d := r.defs[n.Name]
		closures := closureArgs(n)
		if len(closures) == 0 && len(n.Block) == 0 && (r.inlining[n.Name] || len(r.inlining) > 4) {
			return
		}
		sub := *ctx
		if label := nodeLabel(d.Block); label != "" && sub.runner == "" {
			sub.runner = label
			if a := argOf(n.Args, "", 0); a != nil && !a.IsStr && a.Block == nil && label != "node" {
				sub.runner = "node " + a.Raw
			}
		}
		if len(closures) == 0 && !r.inlining[n.Name] && len(r.inlining) <= 4 && (containsStage(d.Block) || !containsStage(n.Block)) {
			r.inlining[n.Name] = true
			r.block(d.Block, &sub)
			delete(r.inlining, n.Name)
		}
		for _, c := range closures {
			r.block(c, &sub)
		}
		r.block(n.Block, &sub)
		ctx.prev = sub.prev
	case len(n.Block) > 0 && (jenkinsWrappers[last] || !jenkinsBuiltins[root] || last == "inside" || last == "withRegistry" || last == "withRun"):
		// A wrapper, Jenkins's or a library's: its closure runs inside it.
		// A library call whose closure runs nothing is configuration
		// (helper.configure { file '…' }), not pipeline.
		if !jenkinsWrappers[last] && !runsSomething(n.Block) {
			if len(n.Chain) == 1 && !jenkinsBuiltins[root] {
				r.libraryStep(n, ctx)
			}
			return
		}
		if !jenkinsBuiltins[root] && len(n.Chain) == 1 && r.inStageless(ctx) {
			r.libraryStep(n, ctx)
		}
		for _, c := range closureArgs(n) {
			r.block(c, ctx)
		}
		r.block(n.Block, ctx)
	default:
		if jenkinsQuiet[n.Name] || root == "env" || root == "currentBuild" || root == "params" {
			return
		}
		if cs := closureArgs(n); len(cs) > 0 && !jenkinsBuiltins[root] {
			// A library call passed closures: they are the pipeline.
			for _, c := range cs {
				r.block(c, ctx)
			}
			return
		}
		// A method call on some object (this.list.add …, executions.put …)
		// is Groovy, not a pipeline step; one on a name the file never
		// defines (infra.runMaven) is a library's.
		if len(n.Chain) > 1 && root != "docker" && !pipelineMethod[last] && (r.locals[root] || jenkinsBuiltins[root]) {
			return
		}
		r.inStepContext(ctx, n.Line)
		r.stepNode(n, ctx, "")
	}
}

// closureArgs lists the closures a call is passed as arguments.
func closureArgs(n *gNode) [][]*gNode {
	var out [][]*gNode
	for _, a := range n.Args {
		if a.Block != nil {
			out = append(out, a.Block)
		}
	}
	return out
}

// Methods on an object that are pipeline steps: an image's build and push.
var pipelineMethod = map[string]bool{"push": true, "build": true, "inside": true, "withRun": true, "run": true, "withRegistry": true, "pull": true}

// runsSomething says whether a closure holds anything a pipeline runs:
// stages, shell steps, nodes, parallel.
func runsSomething(nodes []*gNode) bool {
	found := false
	walkGroovy(nodes, func(n *gNode, _ []*gNode) {
		switch n.Name {
		case "stage", "sh", "bat", "powershell", "pwsh", "node", "parallel", "podTemplate", "checkout", "build", "input":
			found = true
		}
		if n.Kind == "assign" && strings.Contains(n.Name, "[") {
			found = true
		}
		if len(n.Chain) > 1 && n.Chain[len(n.Chain)-1] == "put" {
			found = true
		}
	})
	return found
}

func walkContainers(a *gArg, out *[]string) {
	if a.Block != nil {
		walkGroovy(a.Block, func(n *gNode, _ []*gNode) {
			if n.Name == "containerTemplate" {
				if im := argOf(n.Args, "image", -1); im != nil {
					*out = appendUnique(*out, im.text())
				}
			}
		})
	}
	if strings.HasPrefix(a.Raw, "containerTemplate") {
		if m := regexp.MustCompile(`image\s*:\s*['"]([^'"]+)`).FindStringSubmatch(a.Raw); m != nil {
			*out = appendUnique(*out, m[1])
		}
	}
	for _, it := range a.Items {
		walkContainers(it, out)
	}
}

func nodeLabel(nodes []*gNode) string {
	label := ""
	walkGroovy(nodes, func(n *gNode, _ []*gNode) {
		if label == "" && n.Kind == "call" && (n.Name == "node" || n.Name == "podTemplate") {
			label = argOf(n.Args, "label", 0).text()
			if label == "" {
				label = n.Name
			}
		}
	})
	return label
}

// inStageless says whether statements land outside any stage, where the
// top-level work of a scripted pipeline is.
func (r *jenkinsReader) inStageless(ctx *jctx) bool {
	return r.p.cj == nil || strings.HasPrefix(r.p.cj.ID, "outside stages")
}

// inStepContext makes sure a step has a job: outside every stage, steps go
// into a job named after the file.
func (r *jenkinsReader) inStepContext(ctx *jctx, line int) {
	if r.p.cj == nil {
		j := r.p.job("outside stages", "outside stages", line)
		j.Runner = ctx.runner
		j.Needs = append(j.Needs, ctx.prev...)
		ctx.prev = []string{j.ID}
	}
}

func containsStage(nodes []*gNode) bool {
	found := false
	walkGroovy(nodes, func(n *gNode, _ []*gNode) {
		if n.Kind == "call" && n.Name == "stage" {
			found = true
		}
	})
	return found
}

func containsParallelBranch(nodes []*gNode) bool {
	found := false
	walkGroovy(nodes, func(n *gNode, _ []*gNode) {
		if n.Kind == "assign" && strings.Contains(n.Name, "[") && len(n.Args) == 1 && n.Args[0].Block != nil {
			found = true
		}
	})
	return found
}

func (r *jenkinsReader) stage(n *gNode, ctx *jctx) {
	name := argOf(n.Args, "", 0).text()
	if a := argOf(n.Args, "name", -1); a != nil {
		name = a.text()
	}
	if containsStage(n.Block) || containsParallelOf(n.Block) {
		// A stage of stages: a group, not a job.
		sub := *ctx
		sub.group = joinGroup(ctx.group, name)
		r.block(n.Block, &sub)
		ctx.prev = sub.prev
		return
	}
	j := r.p.job(name, name, n.Line)
	j.Group = ctx.group
	j.Needs = append(j.Needs, ctx.prev...)
	j.Runner = ctx.runner
	j.Condition = ctx.cond
	if ctx.repeat != "" {
		j.Matrix = -1
		if j.Condition == "" {
			j.Condition = "once for each of " + ctx.repeat
		}
	}
	sub := *ctx
	r.block(n.Block, &sub)
	r.envFromName(j)
	// Whatever the stage's body started, the next statement waits for this
	// stage.
	r.p.cj = j
	ctx.prev = []string{j.ID}
	r.p.cj = nil
}

func containsParallelOf(nodes []*gNode) bool {
	for _, n := range nodes {
		if n.Kind == "call" && n.Name == "parallel" {
			return true
		}
	}
	return false
}

// parallel reads a parallel call: named closures, a map, or a variable an
// earlier assignment filled.
func (r *jenkinsReader) parallel(n *gNode, ctx *jctx) {
	var bs []branch
	for _, a := range n.Args {
		switch {
		case a.Key == "failFast":
		case a.Key != "" && a.Block != nil:
			bs = append(bs, branch{key: a.Key, block: a.Block, line: a.Line})
		case len(a.Items) > 0:
			for _, it := range a.Items {
				if it.Key != "" && it.Block != nil {
					bs = append(bs, branch{key: it.Key, block: it.Block, line: it.Line})
				}
			}
		case a.Raw != "" && !a.IsStr:
			name := strings.TrimSpace(a.Raw)
			if got := r.branches[name]; len(got) > 0 {
				bs = append(bs, got...)
			} else {
				r.p.unread(n.Line, n.End, "dynamic_stages")
			}
		}
	}
	if len(n.Block) > 0 {
		// parallel { stage … } inside a scripted script block.
		for _, c := range n.Block {
			if c.Name == "stage" {
				bs = append(bs, branch{key: argOf(c.Args, "", 0).text(), block: []*gNode{c}, line: c.Line})
			}
		}
	}
	start := ctx.prev
	var ends []string
	for _, b := range bs {
		sub := *ctx
		sub.prev = start
		if b.repeat != "" {
			sub.repeat = b.repeat
		}
		key := b.key
		if b.repeat != "" {
			key = "each of " + b.repeat
		}
		if containsStage(b.block) {
			sub.group = joinGroup(ctx.group, key)
			r.block(b.block, &sub)
		} else {
			j := r.p.job(b.key, b.key, b.line)
			j.Group = ctx.group
			j.Needs = append(j.Needs, start...)
			j.Runner = ctx.runner
			if sub.repeat != "" {
				j.Matrix = -1
				j.Condition = "once for each of " + sub.repeat
			}
			r.block(b.block, &sub)
			r.envFromName(j)
			r.p.cj = nil
			sub.prev = []string{j.ID}
		}
		ends = append(ends, sub.prev...)
	}
	if len(ends) > 0 {
		ctx.prev = ends
	}
}

var dockerBuildDSL = regexp.MustCompile(`docker\.build\s*\(\s*(?:"([^"]*)"|'([^']*)')(?:\s*,\s*(?:"([^"]*)"|'([^']*)'))?`)

// steps reads the steps inside a declarative steps block, or a scripted
// stage's body.
func (r *jenkinsReader) steps(nodes []*gNode, ctx *jctx, label string) {
	for _, n := range nodes {
		switch {
		case n.Kind == "call" && n.Name == "script":
			r.block(n.Block, ctx)
		case n.Kind == "call" && (n.Name == "parallel" || n.Name == "stage"):
			r.statement(n, ctx)
		default:
			if n.Kind == "call" {
				r.stepNode(n, ctx, label)
			} else {
				r.statement(n, ctx)
			}
		}
	}
}

// stepNode reads one call as a step.
func (r *jenkinsReader) stepNode(n *gNode, ctx *jctx, label string) {
	p := r.p
	if len(n.Chain) == 0 {
		n.Chain = []string{n.Name}
	}
	root := n.Chain[0]
	last := n.Chain[len(n.Chain)-1]
	name := n.Name
	switch {
	case jenkinsQuiet[name]:
		return
	case name == "sh" || name == "bat" || name == "powershell" || name == "pwsh":
		a := argOf(n.Args, "script", -1)
		if a == nil {
			a = argOf(n.Args, "", 0)
		}
		script := a.text()
		lbl := argOf(n.Args, "label", -1).text()
		if lbl == "" {
			lbl = label
		}
		s := p.step(lbl, script, n.Line)
		_ = s
		p.readShell(p.expand(script), n.Line, ctx.dir)
	case name == "checkout" || name == "git":
		s := p.step(label, name+" "+argsText(n.Args), n.Line)
		s.Verb, s.Tool = "checkout", "git"
	case name == "build":
		job := argOf(n.Args, "job", 0).text()
		s := p.step(label, "build job: "+job, n.Line)
		s.Verb, s.Target = "other", job
		p.handoff("starts_pipeline", job, "", n.Line).Pinned = ""
	case name == "input":
		s := p.step("input", "input "+argOf(n.Args, "message", 0).text(), n.Line)
		s.Verb, s.sure = "approve", true
		if p.cj != nil {
			p.cj.Gate = "approval"
		}
	case name == "load":
		path := argOf(n.Args, "", 0).text()
		s := p.step(label, "load "+path, n.Line)
		s.Target = path
		target := joinPath(ctx.dir, path)
		h := p.handoff("load", target, "", n.Line)
		h.Pinned = ""
		// A script in this repository is read where it is loaded.
		if content, ok := r.lookup(target); ok && !r.loaded[target] && len(r.loaded) < 8 {
			r.loaded[target] = true
			h.Local = target
			saved := p.curFile
			p.curFile = target
			nodes := parseGroovy(string(content))
			for k := range localNames(string(content)) {
				r.locals[k] = true
			}
			r.collect(nodes)
			r.block(nodes, ctx)
			p.curFile = saved
		}
	case root == "docker" && (last == "build" || last == "image"):
		if m := dockerBuildDSL.FindStringSubmatch(n.Raw + n.Name + "(" + argsText(n.Args) + ")"); m != nil {
			r.dockerBuild(m, n.Line, ctx)
		} else if last == "build" {
			a0, a1 := argOf(n.Args, "", 0).text(), argOf(n.Args, "", 1).text()
			r.dockerBuild([]string{"", a0, "", a1, ""}, n.Line, ctx)
		}
		r.block(n.Block, ctx)
	case last == "push" && len(n.Chain) > 1:
		s := p.step(label, n.Name+"("+argsText(n.Args)+")", n.Line)
		s.Verb, s.Tool, s.sure = "publish", "docker", true
		p.pushed = append(p.pushed, pushHit{Raw: argOf(n.Args, "", 0).text(), Line: n.Line, Job: p.jobID()})
	case len(n.Block) > 0 && (jenkinsWrappers[last] || last == "inside" || last == "withRegistry" || last == "withRun"):
		sub := *ctx
		if name == "dir" {
			sub.dir = joinPath(ctx.dir, argOf(n.Args, "", 0).text())
		}
		if name == "container" || name == "node" {
			if c := argOf(n.Args, "", 0).text(); c != "" && p.cj != nil && p.cj.Runner == "" {
				p.cj.Runner = c
			}
		}
		r.steps(n.Block, &sub, label)
	case r.defs[name] != nil:
		r.statement(n, ctx)
	case !jenkinsBuiltins[root] && (len(n.Chain) == 1 || !r.locals[root] && root != "docker"):
		r.libraryStep(n, ctx)
		if len(n.Block) > 0 {
			r.steps(n.Block, ctx, label)
		}
	case len(n.Chain) > 1 && root != "docker" && !pipelineMethod[last]:
		// A method on something the file made: Groovy, not a step.
	case jenkinsBuiltins[root] || len(n.Chain) > 1:
		if root == "env" || root == "currentBuild" || root == "params" || root == "System" || root == "Jenkins" {
			return
		}
		text := n.Name
		if len(n.Args) > 0 {
			text += " " + argsText(n.Args)
		}
		s := p.step(label, text, n.Line)
		switch name {
		case "junit", "jacoco", "recordCoverage", "publishCoverage", "cobertura":
			s.Verb = "test"
		case "archiveArtifacts", "stash", "unstash", "fingerprint":
			s.Verb = "other"
		case "withSonarQubeEnv", "waitForQualityGate", "dependencyCheckPublisher", "recordIssues":
			s.Verb = "scan"
		case "rtUpload", "nexusArtifactUploader", "nexusPublisher", "s3Upload", "googleStorageUpload":
			s.Verb = "publish"
		case "kubernetesDeploy", "ansiblePlaybook":
			s.Verb = "deploy"
		}
		if len(n.Block) > 0 {
			r.steps(n.Block, ctx, label)
		}
	}
}

// libraryStep records a call to a step a shared library defines.
func (r *jenkinsReader) libraryStep(n *gNode, ctx *jctx) {
	p := r.p
	r.inStepContext(ctx, n.Line)
	if p.cj != nil && p.cj.ID == "outside stages" && len(p.cj.Steps) == 0 && len(p.Jobs) == 1 {
		// A Jenkinsfile that is one library call is that call: name its
		// job after it.
		p.cj.Name = n.Name
	}
	s := p.step("", n.Name+"("+argsText(n.Args)+")", n.Line)
	if v := verbOfTemplate(n.Name); v != "other" {
		s.Verb = v
	}
	s.Target = n.Name
	via, ref := r.libraryOf()
	h := p.handoff("library_step", n.Name, ref, n.Line)
	h.Via = via
}

func argsText(args []*gArg) string {
	var parts []string
	for _, a := range args {
		t := a.Raw
		if a.IsStr {
			t = a.Raw
		}
		if a.Block != nil {
			t = "{…}"
		}
		if a.Key != "" {
			t = a.Key + ": " + t
		}
		parts = append(parts, t)
	}
	return strings.Join(parts, ", ")
}

// dockerBuild reads docker.build("name:tag", "-f x/Dockerfile ctx").
func (r *jenkinsReader) dockerBuild(m []string, line int, ctx *jctx) {
	p := r.p
	name := m[1] + m[2]
	args := ""
	if len(m) > 4 {
		args = m[3] + m[4]
	}
	ctxDir, df := ".", ""
	fields := splitShellArgs(args)
	for i := 0; i < len(fields); i++ {
		if fields[i] == "-f" && i+1 < len(fields) {
			df = fields[i+1]
			i++
		} else if !strings.HasPrefix(fields[i], "-") {
			ctxDir = fields[i]
		}
	}
	r.inStepContext(ctx, line)
	if p.cs == nil || p.cs.Line != line {
		s := p.step("", "docker.build "+name, line)
		s.Verb, s.Tool, s.sure = "package", "docker", true
	}
	b := &imageBuild{Names: []ImageName{NormalizeImage(p.expand(name))}, RawTags: []string{name}, BuiltBy: "pipeline", File: p.File, Line: line, Context: joinPath(ctx.dir, ctxDir)}
	if df == "" {
		df = joinPath(ctxDir, "Dockerfile")
	}
	b.Dockerfile = joinPath(ctx.dir, df)
	p.addBuild(b)
}

// envFromName reads the environment a deploy stage names: "Deploy to prod".
func (r *jenkinsReader) envFromName(j *jobFact) {
	if j.Environment != "" {
		return
	}
	deploys := false
	for _, s := range j.Steps {
		if s.Verb == "deploy" {
			deploys = true
		}
	}
	if !deploys {
		return
	}
	if m := jenkinsEnvWord.FindString(j.Name + " " + j.Condition); m != "" {
		j.Environment = strings.ToLower(m)
		for _, d := range r.p.Deploys {
			if d.Job == j.ID && d.Environment == "" {
				d.Environment = j.Environment
			}
		}
	}
}

func (r *jenkinsReader) lookup(path string) ([]byte, bool) {
	if r.rc == nil || r.rc.lookup == nil {
		return nil, false
	}
	return r.rc.lookup(path)
}
