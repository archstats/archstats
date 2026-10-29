package cicd

import (
	"regexp"
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
	tokenFlag     = regexp.MustCompile(`((?i:--(?:token|password|passwd|api-?key|secret|access-?key))[ =]|\slogin\b[^\n]*\s-p\s?|://[^:/\s@]+:)("[^"]+"|'[^']+'|[^\s@]+)`)
)

// scriptFlags reads a script for the risks a line of shell can carry.
func (p *pipelineFact) scriptFlags(script string, line int) {
	if m := curlPipeShell.FindString(script); m != "" {
		p.flag("curl_pipe_shell", oneLine(m, 120), "", line)
	}
	if m := privilegedRun.FindString(script); m != "" {
		p.flag("docker_privileged", oneLine(m, 120), "", line)
	}
	if m := tlsOff.FindString(script); m != "" {
		p.flag("tls_verify_off", strings.TrimSpace(m), "", line)
	}
	for _, m := range tokenFlag.FindAllStringSubmatch(script, -1) {
		v := strings.Trim(m[2], `"'`)
		if isVariable(v) || strings.ContainsAny(v, "$%{") || len(v) < 4 {
			continue
		}
		label := strings.TrimSpace(m[1])
		if strings.Contains(label, "login") {
			label = "login -p"
		}
		if strings.HasPrefix(label, "://") {
			label = "a password in a URL"
		}
		p.flag("token_literal", label+" ***", "", line)
	}
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
			u := strings.ToLower(strings.TrimSpace(last.User))
			switch {
			case u == "":
				add("runs_as_root", "dockerfile", f, "no USER in the final stage", f, last.FromLine)
			case u == "root" || u == "0" || strings.HasPrefix(u, "root:") || strings.HasPrefix(u, "0:"):
				add("runs_as_root", "dockerfile", f, "USER "+last.User, f, last.UserLine)
			}
		}
		ignore := b.hasDockerignore(f)
		for _, st := range df.Stages {
			for _, c := range st.Copies {
				if c.Remote {
					add("add_remote_url", "dockerfile", f, strings.Join(c.Sources, " "), f, c.Line)
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
	// Pipelines.
	for _, p := range b.pipelines {
		subject := func(job string) (string, string) {
			if job == "" {
				return "pipeline", p.ID
			}
			return "job", p.ID + " › " + job
		}
		for _, fl := range p.flags {
			k, s := subject(fl.Job)
			add(fl.Rule, k, s, fl.Detail, p.File, fl.Line)
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
			switch h.Kind {
			case "action", "reusable_workflow", "orb", "pipe", "component":
				if strings.HasPrefix(h.Target, "docker://") && h.Pinned == "none" {
					add("handoff_unpinned", k, s, ref, p.File, h.Line)
				} else if !strings.HasPrefix(h.Target, "docker://") {
					add("handoff_unpinned", k, s, ref, p.File, h.Line)
				}
			case "shared_library":
				add("library_unpinned", "pipeline", p.ID, ref, p.File, h.Line)
			case "include", "template":
				add("remote_include_unpinned", k, s, ref, p.File, h.Line)
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
			if d.Job != "" && isGated(d.Job, 0) {
				continue
			}
			k, s := subject(d.Job)
			add("ungated_prod_deploy", k, s, d.Environment+": "+firstNonEmpty(d.Target, d.Kind), p.File, d.Line)
		}
		// Images pushed only as latest (or with no tag, which is latest).
		tags := map[string][]string{}
		first := map[string]int{}
		note := func(raw string, line int) {
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
				first[n.Repo] = line
			}
		}
		for _, ph := range p.pushed {
			note(ph.Raw, ph.Line)
		}
		for _, bd := range p.Builds {
			if bd.BuiltBy != "pipeline" {
				continue
			}
			pushedHere := false
			for _, j := range p.Jobs {
				for _, s := range j.Steps {
					if j.ID == bd.Job && s.Index == bd.Step && s.Verb == "publish" {
						pushedHere = true
					}
				}
			}
			if pushedHere {
				for _, t := range bd.RawTags {
					note(t, bd.Line)
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
				add("push_latest_only", "pipeline", p.ID, repo, p.File, first[repo])
			}
		}
	}
}
