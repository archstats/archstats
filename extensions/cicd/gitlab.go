package cicd

import (
	stdpath "path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// GitLab CI. A pipeline is the .gitlab-ci.yml and everything it includes,
// merged the way GitLab merges them: included files first, the including
// file over them, `extends` and `default:` filling in what a job leaves out.
// Jobs run by stage unless they say what they need; a job in stage N waits
// for the jobs of stage N-1, which is how they are written out as needs.

var gitlabReserved = map[string]bool{
	"stages": true, "include": true, "variables": true, "default": true, "workflow": true,
	"image": true, "services": true, "before_script": true, "after_script": true, "cache": true, "spec": true,
}

var gitlabDefaultStages = []string{".pre", "build", "test", "deploy", ".post"}

// readCtx is what a reader may ask of the workspace beyond its own file.
type readCtx struct {
	// lookup returns another file's content, by clean workspace path.
	lookup func(path string) ([]byte, bool)
	// glob lists the files a pattern matches, by clean workspace path.
	glob func(pattern string) []string
	// repo is the folder of the repository the file is in ("." for the root).
	repo string
	// repoName is the repository's name: its folder's, or the workspace's.
	repoName string
	// project finds a file of another project in the workspace, by the
	// project's path as GitLab writes it (group/sub/name).
	project func(project, file string) (string, bool)
}

type gitlabEntry struct {
	node *yaml.Node
	file string
	line int
}

type gitlabReader struct {
	p   *pipelineFact
	rc  *readCtx
	top map[string][]gitlabEntry // key -> definitions, lowest precedence first
	// Documents read, to stop include loops.
	seen map[string]bool
	// The file being read, for lines inside included files.
	order []string
}

func readGitLabCI(file string, content []byte, rc *readCtx) *pipelineFact {
	p := newPipeline(file, "gitlab")
	if rc == nil {
		rc = &readCtx{}
	}
	if rc.repo != "" || rc.repoName != "" {
		name := rc.repoName
		if name == "" {
			name = stdpath.Base(rc.repo)
		}
		// $CI_REGISTRY_IMAGE is this project's own registry: an image
		// pushed there is called after the project.
		for _, k := range []string{"CI_REGISTRY_IMAGE", "CI_PROJECT_NAME", "CI_PROJECT_PATH"} {
			p.vars[k] = name
		}
	}
	if rc.repo != "" {
		p.root = rc.repo
	}
	r := &gitlabReader{p: p, rc: rc, top: map[string][]gitlabEntry{}, seen: map[string]bool{}}
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Read = "none"
		p.unread(1, lineCount(content), "unsupported_syntax")
		return p
	}
	doc := docs[0]
	// A file with spec: inputs has its config in the second document.
	if get(doc, "spec") != nil && len(docs) > 1 {
		for _, in := range pairs(at(doc, "spec", "inputs")) {
			if d := str(get(in.Value, "default")); d != "" {
				p.vars["inputs."+in.Key] = d
			}
		}
		doc = docs[1]
	}
	r.load(doc, file, 0)
	if !r.isPipeline() {
		// A file only ever included by others: a template, read as one.
		p.Kind = "template"
	}
	r.read()
	p.finish()
	return p
}

// isPipeline says whether the file stands on its own: a .gitlab-ci.yml, or
// a file with jobs that is not under a templates folder.
func (r *gitlabReader) isPipeline() bool {
	base := strings.ToLower(stdpath.Base(r.p.File))
	return base == ".gitlab-ci.yml" || base == ".gitlab-ci.yaml"
}

// load reads a document and what it includes, includes first.
func (r *gitlabReader) load(doc *yaml.Node, file string, depth int) {
	if r.seen[file] || depth > 5 {
		return
	}
	r.seen[file] = true
	for _, inc := range keysOrItemsList(get(doc, "include")) {
		r.include(inc, file, depth)
	}
	for _, pr := range pairs(doc) {
		if pr.Key == "include" {
			continue
		}
		r.top[pr.Key] = append(r.top[pr.Key], gitlabEntry{node: pr.Value, file: file, line: line(pr.KeyN)})
		if depth == 0 || pr.Key != "workflow" {
			r.order = appendUnique(r.order, pr.Key)
		}
	}
}

// include follows one include entry: local files are read in, everything
// else is a hand-off.
func (r *gitlabReader) include(it *yaml.Node, from string, depth int) {
	p := r.p
	ln := line(it)
	local := ""
	switch {
	case it.Kind == yaml.ScalarNode:
		v := str(it)
		if strings.Contains(v, "://") {
			p.handoff("include", v, refOfURL(v), ln)
			return
		}
		local = v
	case get(it, "local") != nil:
		local = str(get(it, "local"))
	case get(it, "project") != nil:
		project := str(get(it, "project"))
		ref := str(get(it, "ref"))
		files := strs(get(it, "file"))
		for _, f := range files {
			h := p.handoff("include", project+"/"+strings.TrimPrefix(f, "/"), ref, ln)
			if r.rc.project != nil {
				if path, ok := r.rc.project(project, f); ok {
					h.Local = path
					if content, ok := r.rc.lookup(path); ok {
						if docs := yamlDocs(content); len(docs) > 0 {
							r.load(lastDoc(docs), path, depth+1)
						}
					}
				}
			}
		}
		if len(files) == 0 {
			p.handoff("include", project, ref, ln)
		}
		return
	case get(it, "remote") != nil:
		v := str(get(it, "remote"))
		p.handoff("include", v, refOfURL(v), ln)
		return
	case get(it, "template") != nil:
		h := p.handoff("template", str(get(it, "template")), "implicit", ln)
		h.Pinned = "implicit"
		return
	case get(it, "component") != nil:
		target, ref, _ := strings.Cut(str(get(it, "component")), "@")
		p.handoff("component", target, ref, ln)
		return
	default:
		return
	}
	if local == "" {
		return
	}
	var paths []string
	clean := strings.TrimPrefix(local, "/")
	base := r.rc.repo
	if base == "" {
		base = "."
	}
	if strings.ContainsAny(clean, "*?") && r.rc.glob != nil {
		paths = r.rc.glob(joinPath(base, clean))
	} else {
		paths = []string{joinPath(base, clean)}
	}
	found := false
	for _, path := range paths {
		if r.rc.lookup == nil {
			break
		}
		content, ok := r.rc.lookup(path)
		if !ok {
			continue
		}
		found = true
		if docs := yamlDocs(content); len(docs) > 0 {
			r.load(lastDoc(docs), path, depth+1)
		}
	}
	if !found {
		p.unresolvedRef(local, ln, "include_not_found")
	}
}

func lastDoc(docs []*yaml.Node) *yaml.Node {
	if len(docs) > 1 && get(docs[0], "spec") != nil {
		return docs[1]
	}
	return docs[0]
}

var shaInURL = regexp.MustCompile(`/([0-9a-f]{40})/`)

func refOfURL(u string) string {
	if m := shaInURL.FindStringSubmatch(u); m != nil {
		return m[1]
	}
	return ""
}

// value finds a key of a job, through its own definition, extends and
// default:, in that order. GitLab merges hashes deeply; for the keys read
// here the first definition found is the one that counts.
func (r *gitlabReader) value(job, key string, depth int) (*yaml.Node, string) {
	if depth > 10 {
		return nil, ""
	}
	defs := r.top[job]
	for i := len(defs) - 1; i >= 0; i-- {
		if v := get(defs[i].node, key); v != nil {
			return v, defs[i].file
		}
	}
	for i := len(defs) - 1; i >= 0; i-- {
		ext := get(defs[i].node, "extends")
		var parents []string
		if ext != nil && ext.Kind == yaml.ScalarNode {
			parents = []string{str(ext)}
		} else {
			parents = strs(ext)
		}
		// The last parent wins.
		for j := len(parents) - 1; j >= 0; j-- {
			if v, f := r.value(parents[j], key, depth+1); v != nil {
				return v, f
			}
		}
	}
	if depth == 0 {
		switch key {
		case "image", "services", "before_script", "after_script", "tags", "retry", "timeout", "interruptible", "cache", "artifacts":
			if v, f := r.value("default", key, depth+1); v != nil {
				return v, f
			}
			if key != "tags" && key != "retry" && key != "timeout" && key != "interruptible" && key != "artifacts" {
				if defs := r.top[key]; len(defs) > 0 {
					return defs[len(defs)-1].node, defs[len(defs)-1].file
				}
			}
		}
	}
	return nil, ""
}

func (r *gitlabReader) has(job, key string) bool {
	v, _ := r.value(job, key, 0)
	return v != nil
}

func (r *gitlabReader) read() {
	p := r.p
	// Variables: global first, read as the file's own.
	if defs := r.top["variables"]; len(defs) > 0 {
		for _, d := range defs {
			for _, v := range pairs(d.node) {
				p.vars[v.Key] = varValue(v.Value)
			}
		}
	}
	stages := gitlabDefaultStages
	if defs := r.top["stages"]; len(defs) > 0 {
		stages = append([]string{".pre"}, append(strs(defs[len(defs)-1].node), ".post")...)
	}
	stageIndex := map[string]int{}
	for i, s := range stages {
		if _, ok := stageIndex[s]; !ok {
			stageIndex[s] = i
		}
	}
	// Triggers: workflow:rules first, then the rules of every job.
	if defs := r.top["workflow"]; len(defs) > 0 {
		for _, rule := range items(get(defs[len(defs)-1].node, "rules")) {
			r.ruleTriggers(rule)
		}
	}
	type jobDef struct {
		name  string
		stage string
		file  string
		line  int
	}
	var jobs []jobDef
	for _, key := range r.order {
		if gitlabReserved[key] || strings.HasPrefix(key, ".") {
			continue
		}
		defs := r.top[key]
		last := defs[len(defs)-1]
		if last.node == nil || last.node.Kind != yaml.MappingNode {
			continue
		}
		// A job must run something.
		if !r.has(key, "script") && !r.has(key, "trigger") && !r.has(key, "run") {
			continue
		}
		stage := "test"
		if v, _ := r.value(key, "stage", 0); str(v) != "" {
			stage = str(v)
		}
		jobs = append(jobs, jobDef{name: key, stage: stage, file: last.file, line: last.line})
	}
	sort.SliceStable(jobs, func(a, b int) bool { return stageIndex[jobs[a].stage] < stageIndex[jobs[b].stage] })
	jobsInStage := map[string][]string{}
	for _, j := range jobs {
		jobsInStage[j.stage] = append(jobsInStage[j.stage], j.name)
	}
	prevStageJobs := func(stage string) []string {
		idx, ok := stageIndex[stage]
		if !ok {
			return nil
		}
		for i := idx - 1; i >= 0; i-- {
			if js := jobsInStage[stages[i]]; len(js) > 0 {
				return js
			}
		}
		return nil
	}
	sawRules := false
	for _, jd := range jobs {
		j := p.job(jd.name, jd.name, jd.line)
		j.Group = jd.stage
		if jd.file != p.File {
			j.File = jd.file
		}
		p.curFile = j.File
		if v, _ := r.value(jd.name, "needs", 0); v != nil {
			for _, n := range items(v) {
				if s := str(n); s != "" {
					j.Needs = append(j.Needs, s)
					continue
				}
				if proj := str(get(n, "project")); proj != "" {
					p.handoff("starts_pipeline", proj, str(get(n, "ref")), line(n))
					continue
				}
				if s := str(get(n, "job")); s != "" && str(get(n, "pipeline")) == "" {
					j.Needs = append(j.Needs, s)
				}
			}
		} else {
			j.Needs = prevStageJobs(jd.stage)
		}
		// Runner: the image, or the tags that pick one.
		if v, _ := r.value(jd.name, "image", 0); v != nil {
			img := str(v)
			if img == "" {
				img = str(get(v, "name"))
			}
			j.Runner = p.expand(img)
		}
		if v, _ := r.value(jd.name, "services", 0); v != nil {
			for _, s := range items(v) {
				name := str(s)
				if name == "" {
					name = str(get(s, "name"))
				}
				if strings.Contains(name, "dind") {
					j.Runner = strings.TrimPrefix(j.Runner+" · docker-in-docker", " · ")
				}
			}
		}
		if j.Runner == "" {
			if v, _ := r.value(jd.name, "tags", 0); v != nil {
				j.Runner = strings.Join(strs(v), ", ")
			}
		}
		// Matrix.
		if v, _ := r.value(jd.name, "parallel", 0); v != nil {
			if n, err := strconv.Atoi(str(v)); err == nil {
				j.Matrix = n
			} else if m := get(v, "matrix"); m != nil {
				total := 0
				for _, row := range items(m) {
					size := 1
					for _, kv := range pairs(row) {
						if c := len(items(kv.Value)); c > 0 {
							size *= c
						}
					}
					total += size
				}
				j.Matrix = total
			}
		}
		// Environment and gates.
		if v, _ := r.value(jd.name, "environment", 0); v != nil {
			name := str(v)
			if name == "" {
				name = str(get(v, "name"))
			}
			if tier := str(get(v, "deployment_tier")); tier != "" && !strings.Contains(name, tier) {
				name += " (" + tier + ")"
			}
			if action := str(get(v, "action")); action == "stop" {
				name += " · stop"
			}
			j.Environment = name
			j.Gate = "environment"
		}
		var conds []string
		manual := false
		if v, _ := r.value(jd.name, "when", 0); str(v) == "manual" {
			manual = true
		} else if str(v) == "delayed" {
			conds = append(conds, "delayed")
		}
		if v, _ := r.value(jd.name, "rules", 0); v != nil {
			sawRules = true
			for _, rule := range items(v) {
				if c := str(get(rule, "if")); c != "" {
					conds = append(conds, c)
				}
				if ch := strs(get(rule, "changes")); len(ch) > 0 {
					conds = append(conds, "changes: "+strings.Join(ch, ", "))
					p.Paths = append(p.Paths, ch...)
				}
				if str(get(rule, "when")) == "manual" {
					manual = true
				}
				r.ruleTriggers(rule)
			}
		}
		for _, k := range []string{"only", "except"} {
			if v, _ := r.value(jd.name, k, 0); v != nil {
				refs := strs(v)
				if len(refs) == 0 {
					refs = strs(get(v, "refs"))
				}
				if len(refs) > 0 {
					conds = append(conds, k+": "+strings.Join(refs, ", "))
				}
				if k == "only" {
					for _, ref := range refs {
						r.refTrigger(ref, line(v))
					}
				}
			}
		}
		if manual {
			j.Gate = "manual"
		}
		j.Condition = oneLine(strings.Join(conds, " · "), 200)
		if v, _ := r.value(jd.name, "resource_group", 0); str(v) != "" {
			j.Condition = strings.TrimPrefix(j.Condition+" · one at a time: "+str(v), " · ")
		}
		// Job variables, while its scripts are read.
		saved := map[string]string{}
		for k, v := range p.vars {
			saved[k] = v
		}
		if v, _ := r.value(jd.name, "variables", 0); v != nil {
			for _, kv := range pairs(v) {
				p.vars[kv.Key] = varValue(kv.Value)
			}
		}
		// A trigger job starts another pipeline.
		if v, f := r.value(jd.name, "trigger", 0); v != nil {
			r.triggerJob(j, v, f)
		}
		for _, key := range []string{"before_script", "script", "after_script"} {
			v, f := r.value(jd.name, key, 0)
			if v == nil {
				continue
			}
			p.curFile = f
			if f == p.File {
				p.curFile = ""
			}
			label := ""
			if key != "script" {
				label = key
			}
			for _, cmd := range r.scriptLines(v, 0) {
				p.step(label, cmd.text, cmd.line)
				p.readShell(p.expand(cmd.text), cmd.line, p.root)
			}
		}
		p.curFile = ""
		p.vars = saved
	}
	if len(p.Triggers) == 0 {
		// No rules: GitLab runs the pipeline on every push, branch or tag.
		p.trigger("push", "any branch or tag", 1)
		if !sawRules {
			p.trigger("pull_request", "merge requests, when the project runs merge request pipelines", 1)
		}
	}
}

type scriptLine struct {
	text string
	line int
}

// scriptLines flattens a script: nested lists, anchors and !reference tags.
func (r *gitlabReader) scriptLines(n *yaml.Node, depth int) []scriptLine {
	n = resolve(n)
	if n == nil || depth > 6 {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if s := strings.TrimSpace(n.Value); s != "" {
			return []scriptLine{{text: s, line: n.Line}}
		}
	case yaml.SequenceNode:
		if n.Tag == "!reference" {
			path := strs(n)
			if len(path) >= 2 {
				var v *yaml.Node
				defs := r.top[path[0]]
				if len(defs) > 0 {
					v = defs[len(defs)-1].node
					for _, k := range path[1:] {
						v = get(v, k)
					}
				}
				if v != nil {
					return r.scriptLines(v, depth+1)
				}
				r.p.unresolvedRef("!reference "+strings.Join(path, "."), n.Line, "reference_not_found")
			}
			return nil
		}
		var out []scriptLine
		for _, c := range n.Content {
			out = append(out, r.scriptLines(c, depth+1)...)
		}
		return out
	}
	return nil
}

// triggerJob reads `trigger:`: a child pipeline from files in this
// repository, or another project's pipeline.
func (r *gitlabReader) triggerJob(j *jobFact, v *yaml.Node, file string) {
	p := r.p
	ln := line(v)
	s := p.step("trigger", "trigger", ln)
	s.Verb = "other"
	if proj := str(v); proj != "" {
		p.handoff("starts_pipeline", proj, "", ln)
		s.Text, s.Target = "trigger: "+proj, proj
		return
	}
	if proj := str(get(v, "project")); proj != "" {
		h := p.handoff("starts_pipeline", proj, str(get(v, "branch")), ln)
		s.Text, s.Target = "trigger: "+proj, proj
		if r.rc.project != nil {
			if path, ok := r.rc.project(proj, ".gitlab-ci.yml"); ok {
				h.Local = path
			}
		}
		return
	}
	for _, inc := range keysOrItemsList(get(v, "include")) {
		target := str(inc)
		if target == "" {
			target = str(get(inc, "local"))
		}
		if target == "" {
			target = str(get(inc, "artifact"))
			if target != "" {
				h := p.handoff("child_pipeline", target, "", line(inc))
				h.Pinned = ""
				s.Text, s.Target = "trigger: generated "+target, target
				continue
			}
		}
		if target == "" {
			if pr := str(get(inc, "project")); pr != "" {
				for _, f := range strs(get(inc, "file")) {
					p.handoff("child_pipeline", pr+"/"+strings.TrimPrefix(f, "/"), str(get(inc, "ref")), line(inc))
				}
			}
			continue
		}
		base := r.rc.repo
		if base == "" {
			base = "."
		}
		local := joinPath(base, strings.TrimPrefix(target, "/"))
		h := p.handoff("child_pipeline", local, "", line(inc))
		h.Pinned, h.Local = "", local
		s.Text, s.Target = "trigger: "+target, local
	}
}

var (
	ciSource   = regexp.MustCompile(`\$CI_PIPELINE_SOURCE\s*==\s*["']([a-z_]+)["']`)
	ciBranch   = regexp.MustCompile(`\$CI_COMMIT_(BRANCH|REF_NAME)\s*(==|=~)\s*(\S+)`)
	ciTag      = regexp.MustCompile(`\$CI_COMMIT_TAG\b`)
	ciDefaultB = regexp.MustCompile(`\$CI_DEFAULT_BRANCH`)
	ciMR       = regexp.MustCompile(`\$CI_MERGE_REQUEST_`)
)

// ruleTriggers reads what starts a pipeline from a rule's if.
func (r *gitlabReader) ruleTriggers(rule *yaml.Node) {
	p := r.p
	cond := str(get(rule, "if"))
	ln := line(rule)
	if str(get(rule, "when")) == "never" {
		return
	}
	if cond == "" {
		return
	}
	for _, m := range ciSource.FindAllStringSubmatch(cond, -1) {
		switch m[1] {
		case "merge_request_event", "external_pull_request_event":
			p.trigger("pull_request", "merge requests", ln)
		case "schedule":
			p.trigger("schedule", "a schedule set in GitLab", ln)
		case "web", "api", "chat":
			p.trigger("manual", m[1], ln)
		case "pipeline", "parent_pipeline", "trigger", "cross_project_pipeline":
			p.trigger("upstream", m[1], ln)
		case "push":
			p.trigger("push", "", ln)
		}
	}
	if ciMR.MatchString(cond) {
		p.trigger("pull_request", "merge requests", ln)
	}
	if ciTag.MatchString(cond) && !strings.Contains(cond, "$CI_COMMIT_TAG == null") {
		p.trigger("tag", "", ln)
	}
	if m := ciBranch.FindStringSubmatch(cond); m != nil {
		b := strings.Trim(m[3], `"'`)
		if b == "$CI_DEFAULT_BRANCH" {
			b = "default branch"
		}
		p.trigger("push", b, ln)
	} else if ciDefaultB.MatchString(cond) {
		p.trigger("push", "default branch", ln)
	}
}

func (r *gitlabReader) refTrigger(ref string, ln int) {
	switch ref {
	case "merge_requests":
		r.p.trigger("pull_request", "merge requests", ln)
	case "tags":
		r.p.trigger("tag", "", ln)
	case "schedules":
		r.p.trigger("schedule", "a schedule set in GitLab", ln)
	case "web", "api":
		r.p.trigger("manual", ref, ln)
	case "pipelines", "triggers":
		r.p.trigger("upstream", ref, ln)
	case "branches", "pushes":
		r.p.trigger("push", "", ln)
	default:
		r.p.trigger("push", ref, ln)
	}
}

func varValue(n *yaml.Node) string {
	if s := str(n); s != "" {
		return s
	}
	return str(get(n, "value"))
}

func keysOrItemsList(n *yaml.Node) []*yaml.Node {
	n = resolve(n)
	if n == nil {
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		return items(n)
	}
	return []*yaml.Node{n}
}
