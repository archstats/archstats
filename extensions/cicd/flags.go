package cicd

import (
	stdpath "path"
	"regexp"
	"sort"
	"strings"
)

// Hygiene flags are facts that read as risks, each with the file and line
// it was read at. There is no score: a flag says what the file does and the
// UI says, in one sentence, why people care. The rule ids are stable; the
// UI's words and docs/rules/cicd.yml use the same ones.

// The environment names the ungated-deploy rule counts as production. The
// list is shown with the rule, so it is never a hidden judgement.
var productionNames = map[string]bool{"prod": true, "production": true, "prd": true, "live": true}

var (
	curlPipeShell = regexp.MustCompile(`(?i)\b(curl|wget)\b[^\n|;&]*\|\s*(sudo\s+)?(ba|z|da)?sh\b`)
	privilegedRun = regexp.MustCompile(`(?i)\bdocker\s+run\b[^\n;&|]*--privileged\b`)
	tlsOff        = regexp.MustCompile(`(?i)--(insecure-skip-tls-verify|skip-verify|skip-tls-verify|insecure)(=true)?\b|\bcurl\b[^\n;&|]*\s-k\b|GIT_SSL_NO_VERIFY\s*=\s*(1|true)|NODE_TLS_REJECT_UNAUTHORIZED\s*=\s*['"]?0`)
	tokenFlag     = regexp.MustCompile(`((?i:--(?:token|password|passwd|api-?key|secret|access-?key))[ =]|\slogin\b[^\n]*\s-p\s?)("[^"]+"|'[^']+'|[^\s@]+)`)
	// user:password@host. A port is not a password: the @ is what says so.
	urlCredential = regexp.MustCompile(`://[^:/\s@]+:([^\s@/]+)@([^\s/:'"]+)`)
	urlHost       = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://(?:[^\s@/]+@)?(\[[^\]]+\]|[^\s/:'"]+)`)
	// Values written to be replaced, or for an account that lives as long
	// as the job: fakepassword, changeme, <token>.
	placeholderValue = regexp.MustCompile(`(?i)fake|dummy|example|placeholder|changeme|change_me|sample|^x+$|^\*+$|^<.*>$|^test`)
)

// isLoopback says whether a host is this machine: a check against a service
// the job itself started, which no one else can intercept.
func isLoopback(host string) bool {
	h := strings.ToLower(strings.Trim(host, "[]"))
	return h == "localhost" || h == "::1" || h == "0.0.0.0" || strings.HasPrefix(h, "127.") || strings.HasSuffix(h, ".localhost")
}

// commandAt is the shell command around a match: from the separator before
// it to the one after.
func commandAt(script string, start, end int) string {
	from := strings.LastIndexAny(script[:start], "\n;&|") + 1
	to := strings.IndexAny(script[end:], "\n;&|")
	if to < 0 {
		return script[from:]
	}
	return script[from : end+to]
}

// onlyLoopback says whether every host a command names is this machine,
// and it names at least one.
func onlyLoopback(cmd string) bool {
	hosts := urlHost.FindAllStringSubmatch(cmd, -1)
	for _, h := range hosts {
		if !isLoopback(h[1]) {
			return false
		}
	}
	return len(hosts) > 0
}

// scriptFlags reads a script for the risks a line of shell can carry.
func (p *pipelineFact) scriptFlags(script string, line int) {
	if m := curlPipeShell.FindString(script); m != "" {
		p.flag("curl_pipe_shell", oneLine(m, 120), "", line)
	}
	if m := privilegedRun.FindString(script); m != "" {
		p.flag("docker_privileged", oneLine(m, 120), "", line)
	}
	for _, loc := range tlsOff.FindAllStringIndex(script, -1) {
		if onlyLoopback(commandAt(script, loc[0], loc[1])) {
			continue
		}
		p.flag("tls_verify_off", strings.TrimSpace(script[loc[0]:loc[1]]), "", line)
		break
	}
	secret := func(v string) bool {
		v = strings.Trim(v, `"'`)
		return !isVariable(v) && !strings.ContainsAny(v, "$%{") && len(v) >= 4 && !placeholderValue.MatchString(v)
	}
	for _, m := range tokenFlag.FindAllStringSubmatch(script, -1) {
		if !secret(m[2]) {
			continue
		}
		label := strings.TrimSpace(m[1])
		if strings.Contains(label, "login") {
			label = "login -p"
		}
		p.flag("token_literal", label+" ***", "", line)
	}
	for _, m := range urlCredential.FindAllStringSubmatch(script, -1) {
		if secret(m[1]) && !isLoopback(m[2]) {
			p.flag("token_literal", "a password in a URL ***", "", line)
		}
	}
}

// Base images that set a user other than root themselves, so a Dockerfile
// built on them runs as that user without a USER line. Each is documented
// by its publisher: distroless and Chainguard :nonroot tags, Bitnami's
// non-root images, Grafana (uid 472) and the Prometheus images (nobody).
var nonRootBase = regexp.MustCompile(`(?i)(^|[:/-])nonroot([-.:@]|$)|^(docker\.io/)?bitnami/|^(docker\.io/)?grafana/grafana(-oss|-enterprise)?$|^(docker\.io/|quay\.io/)?prom(etheus)?/`)

// devDockerfile says whether a Dockerfile's name marks it as one for working
// on the code rather than shipping it: Dockerfile.dev, dev.Dockerfile,
// Dockerfile.local, Dockerfile.e2e, cypress.Dockerfile.
var devDockerfile = regexp.MustCompile(`(?i)(^|[._-])(dev|develop|development|local|debug|e2e|test|tests|testing|cypress|playwright)([._-]|$)`)

// finalUser follows a Dockerfile's final stage back through the stages it is
// built FROM, to the USER that stage runs as, where it is set and the image
// the chain starts from, and whether any stage on the way says what to run.
func finalUser(df *Dockerfile) (user string, line int, base string, runs bool) {
	st := df.Stages[len(df.Stages)-1]
	for depth := 0; st != nil && depth < len(df.Stages); depth++ {
		runs = runs || st.Entrypoint != ""
		if st.User != "" {
			return st.User, st.UserLine, "", runs
		}
		prev := df.StageByRef(st.From, st.Index)
		if prev == nil {
			return "", 0, st.From, runs
		}
		st = prev
	}
	return "", 0, "", runs
}

func (b *builder) flags() {
	add := func(rule, kind, subject, detail, file string, line int) {
		b.m.Flags = append(b.m.Flags, &FlagRow{Rule: rule, SubjectKind: kind, Subject: subject, Detail: detail, File: file, Line: line})
	}
	// Dockerfiles.
	for _, f := range sortedKeys(b.dockerfiles) {
		if b.skipped(f) {
			continue
		}
		df := b.dockerfiles[f]
		if devDockerfile.MatchString(strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(stdpath.Base(f)), "dockerfile"), "dockerfile")) {
			continue
		}
		for _, st := range df.Stages {
			if strings.EqualFold(st.From, "scratch") || df.StageByRef(st.From, st.Index) != nil || isInterpolated(st.From) {
				continue
			}
			name, tag, digest := splitImageRef(st.From)
			if digest == "" && (tag == "" || tag == "latest") {
				add("base_unpinned_tag", "dockerfile", f, firstNonEmpty(st.From, name), f, st.FromLine)
			} else if digest == "" {
				add("base_no_digest", "dockerfile", f, st.From, f, st.FromLine)
			}
		}
		if n := len(df.Stages); n > 0 {
			last := df.Stages[n-1]
			user, line, base, runs := finalUser(df)
			u := strings.ToLower(strings.TrimSpace(user))
			name, tag, _ := splitImageRef(base)
			switch {
			case u == "" && (base == "" || isInterpolated(base) || nonRootBase.MatchString(name) || nonRootBase.MatchString(":"+tag)):
				// The base image decides, and it is not root, or it cannot
				// be told.
			case u == "" && strings.EqualFold(base, "scratch") && !runs:
				// Files exported with --output: nothing runs in it.
			case u == "":
				if tag != "" {
					name += ":" + tag
				}
				add("runs_as_root", "dockerfile", f, "no USER, on "+name, f, last.FromLine)
			case u == "root" || u == "0" || strings.HasPrefix(u, "root:") || strings.HasPrefix(u, "0:"):
				add("runs_as_root", "dockerfile", f, "USER "+user, f, line)
			}
		}
		ignore := b.hasDockerignore(f)
		for _, st := range df.Stages {
			for _, c := range st.Copies {
				if c.Remote && c.Checksum == "" && !pinnedGitSource(c.RemoteSources) {
					add("add_remote_url", "dockerfile", f, strings.Join(c.RemoteSources, " "), f, c.Line)
				}
				if !ignore && c.From == "" {
					for _, s := range c.Sources {
						if s == "." || s == "./" || s == "*" {
							add("copy_all_no_dockerignore", "dockerfile", f, "COPY "+s+" "+c.Dest, f, c.Line)
							break
						}
					}
				}
			}
			for _, k := range st.Keys {
				if isSecretKey(k.Key) {
					add("secret_like_key", "dockerfile", f, k.Key, f, k.Line)
				}
			}
		}
		for _, k := range df.GlobalKeys {
			if isSecretKey(k.Key) {
				add("secret_like_key", "dockerfile", f, k.Key, f, k.Line)
			}
		}
	}
	// Pipelines. A file GitLab includes is also read as a template of its
	// own; the including pipeline's reading comes first, and the template's
	// says the same thing at the same line.
	pipelines := append([]*pipelineFact{}, b.pipelines...)
	sort.SliceStable(pipelines, func(i, j int) bool { return pipelines[i].Kind != "template" && pipelines[j].Kind == "template" })
	seen := map[string]bool{}
	add = func(rule, kind, subject, detail, file string, line int) {
		k := rule + "|" + file + "|" + itoa(line) + "|" + detail
		if seen[k] {
			return
		}
		seen[k] = true
		b.m.Flags = append(b.m.Flags, &FlagRow{Rule: rule, SubjectKind: kind, Subject: subject, Detail: detail, File: file, Line: line})
	}
	for _, p := range pipelines {
		if p.copyOnly {
			continue
		}
		subject := func(job string) (string, string) {
			if job == "" {
				return "pipeline", p.ID
			}
			return "job", p.ID + " › " + job
		}
		for _, fl := range p.flags {
			k, s := subject(fl.Job)
			add(fl.Rule, k, s, fl.Detail, firstNonEmpty(fl.File, p.File), fl.Line)
		}
		for _, h := range p.Handoffs {
			if h.Local != "" || h.Pinned == "" || h.Pinned == "sha" || h.Pinned == "version" || h.Pinned == "implicit" {
				continue
			}
			k, s := subject(h.Job)
			ref := h.Target
			if h.Ref != "" {
				ref += "@" + h.Ref
			}
			file := firstNonEmpty(h.File, p.File)
			switch h.Kind {
			case "action", "reusable_workflow", "orb", "pipe", "component":
				if strings.HasPrefix(h.Target, "docker://") && h.Pinned == "none" {
					add("handoff_unpinned", k, s, ref, file, h.Line)
				} else if !strings.HasPrefix(h.Target, "docker://") {
					add("handoff_unpinned", k, s, ref, file, h.Line)
				}
			case "shared_library":
				// With no @ref the server's default version is loaded,
				// which is set on the server, not in this file.
				if h.Pinned == "branch" {
					add("library_unpinned", "pipeline", p.ID, ref, file, h.Line)
				}
			case "include", "template":
				add("remote_include_unpinned", k, s, ref, file, h.Line)
			}
		}
		// A deploy to production with nothing in front of it.
		gated := map[string]bool{}
		var isGated func(id string, depth int) bool
		isGated = func(id string, depth int) bool {
			if v, ok := gated[id]; ok {
				return v
			}
			j := p.jobByID(id)
			if j == nil || depth > 50 {
				return false
			}
			g := j.Gate == "manual" || j.Gate == "approval" || j.Gate == "environment"
			for _, s := range j.Steps {
				if s.Verb == "approve" {
					g = true
				}
			}
			for _, n := range j.Needs {
				if !g && isGated(n, depth+1) {
					g = true
				}
			}
			gated[id] = g
			return g
		}
		for _, d := range p.Deploys {
			env := strings.ToLower(d.Environment)
			if !productionNames[env] {
				continue
			}
			// A deploy in a workflow outside the workspace may hold its
			// gate there; a Fleet target ships what is merged, and the
			// merge is the gate. Neither file can show there is none.
			if d.Kind == "delegated" || d.Platform == "fleet" {
				continue
			}
			if d.Job != "" && isGated(d.Job, 0) {
				continue
			}
			k, s := subject(d.Job)
			add("ungated_prod_deploy", k, s, d.Environment+": "+firstNonEmpty(d.Target, d.Kind), firstNonEmpty(d.File, p.File), d.Line)
		}
		// Images pushed only as latest (or with no tag, which is latest).
		tags := map[string][]string{}
		first := map[string]int{}
		firstFile := map[string]string{}
		note := func(raw, file string, line int) {
			n := NormalizeImage(p.expand(raw))
			if n.Repo == "" {
				return
			}
			_, tag, digest := splitImageRef(raw)
			if digest != "" {
				tag = "@" + digest
			}
			if isInterpolated(tag) {
				tag = "$"
			}
			tags[n.Repo] = append(tags[n.Repo], tag)
			if _, ok := first[n.Repo]; !ok {
				first[n.Repo], firstFile[n.Repo] = line, file
			}
		}
		for _, ph := range p.pushed {
			note(ph.Raw, ph.File, ph.Line)
		}
		for _, bd := range p.Builds {
			if bd.BuiltBy != "pipeline" {
				continue
			}
			pushedHere, file := false, ""
			for _, j := range p.Jobs {
				for _, s := range j.Steps {
					if j.ID == bd.Job && s.Index == bd.Step && s.Verb == "publish" {
						pushedHere, file = true, firstNonEmpty(s.File, j.File)
					}
				}
			}
			if pushedHere {
				for _, t := range bd.RawTags {
					note(t, file, bd.Line)
				}
			}
		}
		for _, repo := range sortedKeys(tags) {
			latestOnly := true
			for _, t := range tags[repo] {
				if t != "" && t != "latest" {
					latestOnly = false
				}
			}
			if latestOnly {
				add("push_latest_only", "pipeline", p.ID, repo, firstNonEmpty(firstFile[repo], p.File), first[repo])
			}
		}
	}
}

// pinnedGitSource says whether every remote ADD source is a git repository
// at a commit: git@host:repo.git#<sha>.
func pinnedGitSource(srcs []string) bool {
	for _, s := range srcs {
		_, ref, ok := strings.Cut(s, "#")
		if !ok || !shaRef.MatchString(ref) {
			return false
		}
	}
	return len(srcs) > 0
}
