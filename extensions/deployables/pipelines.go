package deployables

import (
	"fmt"
	stdpath "path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// A pipeline is read for five things an architect asks of it: what starts
// it, what part of the tree it watches, what it does (build, test, scan,
// package, publish, deploy, approve), what it builds and deploys, and
// whether it does the work itself or hands it to a shared template
// somewhere else. In the client workspace, every one of 26 repositories
// hands its deployment to one central workflow repository at `@main`.

// The stages, in the order a strip shows them.
var stageOrder = []string{"build", "test", "scan", "package", "publish", "deploy", "approve"}

type pipelineFact struct {
	ID       string
	Name     string
	System   string // github_actions, gitlab, jenkins, azure_pipelines, circleci, travis, bitbucket, other
	File     string
	Line     int
	Parsed   string // full, partial, none
	Triggers []string
	Paths    []string
	stages   map[string]int // stage -> first line
	Tools    map[string]bool
	// A remote template the work is handed to: a reusable workflow, a
	// Jenkins shared library, a GitLab include.
	Delegates []delegation
	// Local pipelines this one calls.
	Calls        []string
	Environments []envHit
	Builds       []*imageBuild
	Deploys      []deployHit
	Runtimes     []runtimeHit
	// Build steps that run a build tool (mvn, gradle, npm, dotnet), which
	// build whatever module the repository holds.
	BuildTool     bool
	BuildToolLine int
}

type delegation struct {
	Target string
	Ref    string
	Line   int
	Stage  string
}

type envHit struct {
	Name string
	Kind string // enumerated, pattern
	Line int
}

type deployHit struct {
	Kind   string // helm, kubectl, kustomize, compose, skaffold, sam, serverless, cloud, delegated
	Target string // a chart or manifest path, a service name, an image
	Line   int
}

type runtimeHit struct {
	Tool, Version string
	Line          int
}

func newPipeline(file, system string) *pipelineFact {
	return &pipelineFact{ID: file, File: file, Line: 1, System: system, Name: stdpath.Base(file), Parsed: "full", stages: map[string]int{}, Tools: map[string]bool{}}
}

func (p *pipelineFact) hit(stage string, line int) {
	if stage == "" {
		return
	}
	if l, ok := p.stages[stage]; !ok || (line > 0 && line < l) {
		p.stages[stage] = line
	}
}

// Stages lists what the pipeline does, in strip order.
func (p *pipelineFact) Stages() []string {
	var out []string
	for _, s := range stageOrder {
		if _, ok := p.stages[s]; ok {
			out = append(out, s)
		}
	}
	return out
}

func (p *pipelineFact) ToolList() []string { return sortedKeys(p.Tools) }

// ---------------------------------------------------------------------------
// Classifying a step
// ---------------------------------------------------------------------------

var stageRules = []struct {
	stage string
	re    *regexp.Regexp
}{
	{"scan", regexp.MustCompile(`(?i)codeql|sonar|trivy|snyk|grype|anchore|checkmarx|fortify|dependency-check|owasp|semgrep|gitleaks|zizmor|scorecard|nexus-?iq|blackduck|\bfoss\b|foss-|mend|whitesource|veracode|\bsast\b|static-analysis|dependency-review|license-check|secret-detection|\baudit\b`)},
	{"test", regexp.MustCompile(`(?i)\btests?\b|pytest|\bjest\b|vitest|go test|mvnw? .*\bverify\b|gradlew? .*\btest|\bcheck\b|npm (run )?test|yarn test|pnpm test|cypress|playwright|junit|karma|phpunit|rspec|\btox\b|\bnox\b|dotnet test|coverage|\be2e\b|integration-tests?`)},
	{"package", regexp.MustCompile(`(?i)docker (buildx )?build|buildx|build-push-action|\bjib\b|bootBuildImage|build-image|kaniko|buildah|podman build|pack build|ko build|compose build|goreleaser|helm package|npm pack|sam build|docker/bake|\bbake\b`)},
	{"publish", regexp.MustCompile(`(?i)docker push|login-action|docker login|\bpublish\b|twine upload|npm publish|mvnw? .*\bdeploy\b|gradlew? .*publish|chart-releaser|push-to-registry|ecr-login|gcr|artifactory|jfrog|nexus-upload`)},
	{"deploy", regexp.MustCompile(`(?i)helm (upgrade|install|rollback)|kubectl (apply|set image|rollout|create|replace|delete)|kustomize build|argocd|\bflux\b|skaffold (run|deploy)|terraform apply|sam deploy|serverless deploy|sls deploy|gcloud (run|app|functions) deploy|deploy-cloudrun|az (webapp|containerapp|functionapp)|webapps-deploy|k8s-deploy|aws (ecs|lambda|cloudformation)|ecs-deploy|cf push|fly deploy|vercel|netlify deploy|heroku|ansible-playbook|\bdeploy|rollback|release-to|udeploy`)},
	{"build", regexp.MustCompile(`(?i)\bmvnw?\b|\bgradlew?\b|npm (ci|install|run build)|yarn( install| build|$)|pnpm (install|build|i\b)|go build|dotnet (build|publish|restore)|cargo build|\bmake\b|setup-(java|node|go|python|dotnet)|\bant\b|\bsbt\b|bazel build|\btsc\b|composer install|pip install|poetry install`)},
	{"approve", regexp.MustCompile(`(?i)manual-approval|\bapproval\b|go-?nogo|change-request|\bapprove\b|input message`)},
}

var toolRules = map[string]*regexp.Regexp{
	"maven":      regexp.MustCompile(`(?i)\bmvnw?\b|setup-java.*maven|build-maven|-maven\b`),
	"gradle":     regexp.MustCompile(`(?i)\bgradlew?\b|gradle-build-action|setup-gradle|build-gradle`),
	"npm":        regexp.MustCompile(`(?i)\bnpm\b|\byarn\b|\bpnpm\b|setup-node`),
	"go":         regexp.MustCompile(`(?i)\bgo (build|test|vet|mod)\b|setup-go`),
	"dotnet":     regexp.MustCompile(`(?i)\bdotnet\b|setup-dotnet`),
	"python":     regexp.MustCompile(`(?i)\bpip\b|\bpoetry\b|\bpytest\b|setup-python|\buv (sync|run)`),
	"docker":     regexp.MustCompile(`(?i)\bdocker\b|buildx|build-push-action|login-action`),
	"helm":       regexp.MustCompile(`(?i)\bhelm\b|setup-helm`),
	"kubectl":    regexp.MustCompile(`(?i)\bkubectl\b|k8s-deploy|setup-kubectl`),
	"kustomize":  regexp.MustCompile(`(?i)\bkustomize\b`),
	"terraform":  regexp.MustCompile(`(?i)\bterraform\b|setup-terraform|\btofu\b`),
	"skaffold":   regexp.MustCompile(`(?i)\bskaffold\b`),
	"sam":        regexp.MustCompile(`(?i)\bsam (build|deploy)\b|setup-sam`),
	"serverless": regexp.MustCompile(`(?i)\bserverless\b|\bsls\b`),
	"gcloud":     regexp.MustCompile(`(?i)\bgcloud\b|google-github-actions`),
	"az":         regexp.MustCompile(`(?i)\baz (webapp|containerapp|login|aks|acr)|azure/`),
	"aws":        regexp.MustCompile(`(?i)\baws (ecs|ecr|s3|lambda|cloudformation|eks)|aws-actions/`),
	"jib":        regexp.MustCompile(`(?i)\bjib\b`),
	"codeql":     regexp.MustCompile(`(?i)codeql`),
	"sonar":      regexp.MustCompile(`(?i)sonar`),
	"trivy":      regexp.MustCompile(`(?i)\btrivy\b`),
	"ansible":    regexp.MustCompile(`(?i)ansible`),
	"argocd":     regexp.MustCompile(`(?i)argocd`),
}

// classifyText adds every stage and tool a piece of pipeline text shows.
func (p *pipelineFact) classifyText(text string, line int) {
	for _, r := range stageRules {
		if r.re.MatchString(text) {
			p.hit(r.stage, line)
		}
	}
	for tool, re := range toolRules {
		if re.MatchString(text) {
			p.Tools[tool] = true
		}
	}
}

// A reusable template is classified by its name: build-maven.yml builds,
// deployment-nonproduction.yml deploys, codeQL.yml scans.
func stageOfTemplate(name string) []string {
	n := strings.ToLower(stdpath.Base(name))
	words := strings.NewReplacer("-", " ", "_", " ", ".yml", "", ".yaml", "").Replace(n)
	found := map[string]bool{}
	for _, r := range stageRules {
		if r.re.MatchString(words) || r.re.MatchString(n) {
			found[r.stage] = true
		}
	}
	// A template's name is a label, and "build" in it means what it says;
	// in a step's display name ("Build and push") it does not.
	for _, w := range strings.Fields(words) {
		if w == "build" || w == "compile" || w == "ci" {
			found["build"] = true
		}
	}
	var out []string
	for _, st := range stageOrder {
		if found[st] {
			out = append(out, st)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Commands inside a step
// ---------------------------------------------------------------------------

var (
	dockerBuildCmd  = regexp.MustCompile(`(?m)\b(?:docker|podman)\s+(?:buildx\s+)?build\s+([^\n;&|]+)`)
	helmDeployCmd   = regexp.MustCompile(`(?m)\bhelm\s+(?:upgrade\s+--install|upgrade|install)\s+([^\n;&|]+)`)
	kubectlApplyCmd = regexp.MustCompile(`(?m)\bkubectl\s+(?:apply|create|replace)\s+([^\n;&|]+)`)
	kustomizeCmd    = regexp.MustCompile(`(?m)\bkustomize\s+build\s+(\S+)`)
	cloudRunCmd     = regexp.MustCompile(`(?m)\bgcloud\s+(?:beta\s+)?run\s+(?:deploy|jobs\s+(?:deploy|update))\s+([A-Za-z0-9_$.{}-]+)([^\n;&|]*)`)
	lambdaCmd       = regexp.MustCompile(`(?m)--function-name[ =](\S+)`)
	ecsCmd          = regexp.MustCompile(`(?m)\baws\s+ecs\s+update-service[^\n]*--service[ =](\S+)`)
	composeCmd      = regexp.MustCompile(`(?m)\bdocker[ -]compose\b[^\n]*\b(build|up)\b`)
	skaffoldCmd     = regexp.MustCompile(`(?m)\bskaffold\s+(run|deploy|build)\b`)
	samCmd          = regexp.MustCompile(`(?m)\b(sam|serverless|sls)\s+deploy\b`)
	jibCmd          = regexp.MustCompile(`(?m)\bjib(:build|:dockerBuild|Build|DockerBuild)?\b`)
	buildToolCmd    = regexp.MustCompile(`(?m)\b(mvnw?|gradlew?|\./mvnw|\./gradlew)\b[^\n]*\b(package|install|verify|build|bootJar|assemble|jib)\b|\bdotnet\s+(publish|build)\b|\bnpm\s+run\s+build\b|\bgo\s+build\b`)
)

// joinContinuations turns shell line continuations into one line, keeping
// a map from the joined text back to line offsets is not worth it; the step's
// line is used for everything found in it.
func joinContinuations(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\\\r\n", " "), "\\\n", " ")
}

func splitShellArgs(s string) []string {
	var out []string
	var cur strings.Builder
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ' ' || c == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// readShell reads a script for builds, deploys and build tools. dir is the
// directory commands run in, for resolving relative paths.
func (p *pipelineFact) readShell(script string, line int, dir string) {
	script = joinContinuations(script)
	// An expression is one argument however it is spaced:
	// `pr-${{ github.event.number }}` must not split into three.
	script = ghExpr.ReplaceAllStringFunc(script, func(m string) string { return strings.ReplaceAll(m, " ", "") })
	p.classifyText(script, line)
	for _, m := range dockerBuildCmd.FindAllStringSubmatch(script, -1) {
		args := splitShellArgs(m[1])
		b := &imageBuild{BuiltBy: "pipeline", File: p.File, Line: line}
		df := ""
		ctx := ""
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "-t" || a == "--tag":
				if i+1 < len(args) {
					b.Names = append(b.Names, NormalizeImage(args[i+1]))
					i++
				}
			case strings.HasPrefix(a, "--tag="), strings.HasPrefix(a, "-t="):
				b.Names = append(b.Names, NormalizeImage(a[strings.Index(a, "=")+1:]))
			case a == "-f" || a == "--file":
				if i+1 < len(args) {
					df = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--file="):
				df = a[len("--file="):]
			case a == "--target":
				if i+1 < len(args) {
					b.Target = args[i+1]
					i++
				}
			case strings.HasPrefix(a, "--target="):
				b.Target = a[len("--target="):]
			case strings.HasPrefix(a, "--") && !strings.Contains(a, "="):
				// A flag with a separate value: skip the value.
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && a != "--push" && a != "--load" && a != "--pull" && a != "--no-cache" && a != "--rm" && a != "--quiet" {
					i++
				}
			case strings.HasPrefix(a, "-"):
			default:
				ctx = a
			}
		}
		if ctx == "" || isInterpolated(ctx) {
			continue
		}
		// An untagged build names nothing a deployment could run, and a
		// sentence that mentions `docker build` is not one.
		named := false
		for _, n := range b.Names {
			if n.Repo != "" {
				named = true
			}
		}
		if !named {
			continue
		}
		b.Context = joinPath(dir, ctx)
		if b.Context == "" {
			continue
		}
		if df == "" {
			df = stdpath.Join(ctx, "Dockerfile")
			b.Dockerfile = joinPath(dir, df)
		} else {
			b.Dockerfile = joinPath(dir, df)
		}
		p.Builds = append(p.Builds, b)
	}
	for _, m := range helmDeployCmd.FindAllStringSubmatch(script, -1) {
		args := splitShellArgs(m[1])
		var pos []string
		for i := 0; i < len(args); i++ {
			if strings.HasPrefix(args[i], "-") {
				if !strings.Contains(args[i], "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && args[i] != "--install" && args[i] != "--wait" && args[i] != "--atomic" && args[i] != "--create-namespace" && args[i] != "--debug" {
					if args[i] == "-f" || args[i] == "--values" {
						p.Deploys = append(p.Deploys, deployHit{Kind: "helm_values", Target: joinPath(dir, args[i+1]), Line: line})
					}
					i++
				}
				continue
			}
			pos = append(pos, args[i])
		}
		if len(pos) >= 2 {
			p.Deploys = append(p.Deploys, deployHit{Kind: "helm", Target: joinPath(dir, pos[1]), Line: line})
		}
	}
	for _, m := range kubectlApplyCmd.FindAllStringSubmatch(script, -1) {
		args := splitShellArgs(m[1])
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "-f" || args[i] == "--filename" || args[i] == "-k" || args[i] == "--kustomize" {
				p.Deploys = append(p.Deploys, deployHit{Kind: "kubectl", Target: joinPath(dir, args[i+1]), Line: line})
			}
		}
	}
	for _, m := range kustomizeCmd.FindAllStringSubmatch(script, -1) {
		p.Deploys = append(p.Deploys, deployHit{Kind: "kustomize", Target: joinPath(dir, m[1]), Line: line})
	}
	for _, m := range cloudRunCmd.FindAllStringSubmatch(script, -1) {
		p.Deploys = append(p.Deploys, deployHit{Kind: "cloud", Target: m[1], Line: line})
	}
	for _, m := range lambdaCmd.FindAllStringSubmatch(script, -1) {
		p.Deploys = append(p.Deploys, deployHit{Kind: "cloud", Target: m[1], Line: line})
	}
	for _, m := range ecsCmd.FindAllStringSubmatch(script, -1) {
		p.Deploys = append(p.Deploys, deployHit{Kind: "cloud", Target: m[1], Line: line})
	}
	for _, m := range composeCmd.FindAllStringSubmatch(script, -1) {
		if m[1] == "up" {
			p.Deploys = append(p.Deploys, deployHit{Kind: "compose", Line: line})
		}
	}
	for _, m := range skaffoldCmd.FindAllStringSubmatch(script, -1) {
		if m[1] != "deploy" {
			// skaffold build and skaffold run build every artifact.
			p.Builds = append(p.Builds, &imageBuild{BuiltBy: "skaffold", File: p.File, Line: line})
		}
		if m[1] != "build" {
			p.Deploys = append(p.Deploys, deployHit{Kind: "skaffold", Line: line})
		}
	}
	if samCmd.MatchString(script) {
		p.Deploys = append(p.Deploys, deployHit{Kind: "sam", Line: line})
	}
	if jibCmd.MatchString(script) && (strings.Contains(script, "mvn") || strings.Contains(script, "gradle")) {
		p.Builds = append(p.Builds, &imageBuild{BuiltBy: "jib", File: p.File, Line: line})
	}
	if buildToolCmd.MatchString(script) && !p.BuildTool {
		p.BuildTool, p.BuildToolLine = true, line
	}
}

// ---------------------------------------------------------------------------
// GitHub Actions
// ---------------------------------------------------------------------------

var (
	ghExpr       = regexp.MustCompile(`\$\{\{\s*([^}]+?)\s*\}\}`)
	envInputRef  = regexp.MustCompile(`(?i)inputs\.([a-z0-9_-]+)`)
	versionInput = regexp.MustCompile(`(?i)^(java|node|go|python|dotnet|jdk)[-_]?version$`)
	envInputName = regexp.MustCompile(`(?i)^(env|environment|stage|target[-_]?env|deploy[-_]?env|environment[-_]?name)$`)
	prNumberRef  = regexp.MustCompile(`(?i)github\.event\.(number|pull_request\.number)|pr[-_]?number|pull_request\.head|preview`)
)

func readGitHubWorkflow(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "github_actions")
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Parsed = "none"
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
	switch {
	case on == nil:
	case on.Kind == yaml.ScalarNode:
		p.Triggers = append(p.Triggers, str(on))
	case on.Kind == yaml.SequenceNode:
		p.Triggers = append(p.Triggers, strs(on)...)
	case on.Kind == yaml.MappingNode:
		for _, t := range pairs(on) {
			p.Triggers = append(p.Triggers, t.Key)
			for _, g := range strs(get(t.Value, "paths")) {
				p.Paths = append(p.Paths, g)
			}
			for _, g := range strs(get(t.Value, "paths-ignore")) {
				p.Paths = append(p.Paths, "!"+g)
			}
		}
	}
	dispatchEnvs := map[string][]string{}
	for _, in := range pairs(at(on, "workflow_dispatch", "inputs")) {
		opts := strs(get(in.Value, "options"))
		if envInputName.MatchString(in.Key) && len(opts) > 0 {
			dispatchEnvs[in.Key] = opts
			for _, o := range opts {
				p.Environments = append(p.Environments, envHit{Name: o, Kind: "enumerated", Line: line(in.KeyN)})
			}
		}
		if regexp.MustCompile(`(?i)go-?no-?go|change[-_]?request|approv`).MatchString(in.Key) {
			p.hit("approve", line(in.KeyN))
		}
	}
	workflowEnv := map[string]string{}
	for _, e := range envEntries(get(doc, "env")) {
		workflowEnv[e.Key] = e.Value
	}
	isPR := false
	for _, t := range p.Triggers {
		if t == "pull_request" || t == "pull_request_target" {
			isPR = true
		}
	}
	for _, job := range pairs(get(doc, "jobs")) {
		j := job.Value
		jobEnv := map[string]string{}
		for k, v := range workflowEnv {
			jobEnv[k] = v
		}
		for _, e := range envEntries(get(j, "env")) {
			jobEnv[e.Key] = e.Value
		}
		combos := matrixCombos(at(j, "strategy", "matrix"))
		// A reusable workflow: the job is the call.
		if uses := str(get(j, "uses")); uses != "" {
			ln := line(get(j, "uses"))
			target, ref, _ := strings.Cut(uses, "@")
			if strings.HasPrefix(uses, "./") {
				p.Calls = append(p.Calls, joinPath(".", strings.TrimPrefix(target, "./")))
			} else {
				p.Delegates = append(p.Delegates, delegation{Target: target, Ref: ref, Line: ln})
			}
			for _, st := range stageOfTemplate(target) {
				p.hit(st, ln)
				if st == "deploy" {
					p.Deploys = append(p.Deploys, deployHit{Kind: "delegated", Target: target, Line: ln})
				}
				if st == "build" || st == "package" {
					if !p.BuildTool {
						p.BuildTool, p.BuildToolLine = true, ln
					}
				}
			}
			p.classifyText(target, ln)
			for _, w := range pairs(get(j, "with")) {
				if versionInput.MatchString(w.Key) && str(w.Value) != "" {
					p.Runtimes = append(p.Runtimes, runtimeHit{Tool: runtimeTool(w.Key), Version: str(w.Value), Line: line(w.KeyN)})
				}
			}
		}
		// The environment a job deploys to.
		if env := get(j, "environment"); env != nil {
			name := str(env)
			if name == "" {
				name = str(get(env, "name"))
			}
			ln := line(env)
			switch {
			case name == "":
			case prNumberRef.MatchString(name):
				p.Environments = append(p.Environments, envHit{Name: "per pull request", Kind: "pattern", Line: ln})
			case strings.Contains(name, "${{"):
				// ${{ inputs.environment }} is every option the input offers,
				// which were recorded above.
				if m := envInputRef.FindStringSubmatch(name); m != nil && len(dispatchEnvs[m[1]]) > 0 {
					break
				}
				if len(combos) > 0 {
					for _, c := range combos {
						if v := expandGH(name, c, jobEnv); !strings.Contains(v, "${{") {
							p.Environments = append(p.Environments, envHit{Name: v, Kind: "enumerated", Line: ln})
						}
					}
				}
			default:
				p.Environments = append(p.Environments, envHit{Name: name, Kind: "enumerated", Line: ln})
			}
		}
		if len(combos) == 0 {
			combos = []map[string]interface{}{nil}
		}
		workdir := str(at(j, "defaults", "run", "working-directory"))
		for _, st := range items(get(j, "steps")) {
			ln := line(st)
			uses := str(get(st, "uses"))
			run := str(get(st, "run"))
			name := str(get(st, "name"))
			dir := joinPath(".", workdir)
			if wd := str(get(st, "working-directory")); wd != "" {
				dir = joinPath(".", wd)
			}
			if dir == "" {
				dir = "."
			}
			p.classifyText(uses+"\n"+name, ln)
			with := get(st, "with")
			for _, w := range pairs(with) {
				if versionInput.MatchString(w.Key) && str(w.Value) != "" && !strings.Contains(str(w.Value), "${{") {
					p.Runtimes = append(p.Runtimes, runtimeHit{Tool: runtimeTool(w.Key + " " + uses), Version: str(w.Value), Line: line(w.KeyN)})
				}
			}
			for _, combo := range combos {
				if run != "" {
					p.readShell(expandGH(run, combo, jobEnv), ln, dir)
				}
				if strings.HasPrefix(uses, "docker/build-push-action") {
					b := &imageBuild{BuiltBy: "pipeline", File: file, Line: ln}
					ctx := expandGH(str(get(with, "context")), combo, jobEnv)
					if ctx == "" {
						ctx = "."
					}
					if isInterpolated(ctx) {
						continue
					}
					b.Context = joinPath(".", ctx)
					df := expandGH(str(get(with, "file")), combo, jobEnv)
					if df == "" {
						df = stdpath.Join(ctx, "Dockerfile")
					}
					b.Dockerfile = joinPath(".", df)
					b.Target = expandGH(str(get(with, "target")), combo, jobEnv)
					for _, t := range splitList(expandGH(str(get(with, "tags")), combo, jobEnv)) {
						b.Names = append(b.Names, NormalizeImage(t))
					}
					// Tags from docker/metadata-action name the image in its `images`.
					for _, other := range items(get(j, "steps")) {
						if strings.HasPrefix(str(get(other, "uses")), "docker/metadata-action") {
							for _, im := range splitList(expandGH(str(at(other, "with", "images")), combo, jobEnv)) {
								b.Names = append(b.Names, NormalizeImage(im))
							}
						}
					}
					if b.Context != "" {
						p.Builds = append(p.Builds, b)
					}
				}
				switch {
				case strings.HasPrefix(uses, "azure/k8s-deploy"):
					for _, m := range splitList(expandGH(str(get(with, "manifests")), combo, jobEnv)) {
						p.Deploys = append(p.Deploys, deployHit{Kind: "kubectl", Target: joinPath(".", m), Line: ln})
					}
				case strings.HasPrefix(uses, "google-github-actions/deploy-cloudrun"):
					p.Deploys = append(p.Deploys, deployHit{Kind: "cloud", Target: expandGH(str(get(with, "service"))+str(get(with, "job")), combo, jobEnv), Line: ln})
				case strings.HasPrefix(uses, "azure/webapps-deploy"), strings.HasPrefix(uses, "azure/functions-action"):
					p.Deploys = append(p.Deploys, deployHit{Kind: "cloud", Target: expandGH(str(get(with, "app-name")), combo, jobEnv), Line: ln})
				case strings.HasPrefix(uses, "aws-actions/amazon-ecs-deploy-task-definition"):
					p.Deploys = append(p.Deploys, deployHit{Kind: "cloud", Target: expandGH(str(get(with, "service")), combo, jobEnv), Line: ln})
				case strings.HasPrefix(uses, "./") && strings.Contains(uses, ".github/workflows"):
					p.Calls = append(p.Calls, joinPath(".", strings.TrimPrefix(uses, "./")))
				case strings.HasPrefix(uses, "./"):
					// A local composite action: part of this pipeline.
				case strings.Contains(uses, "/.github/workflows/"):
					target, ref, _ := strings.Cut(uses, "@")
					p.Delegates = append(p.Delegates, delegation{Target: target, Ref: ref, Line: ln})
				}
			}
		}
	}
	if isPR && len(p.Deploys) > 0 && prNumberRef.MatchString(string(content)) {
		p.Environments = append(p.Environments, envHit{Name: "per pull request", Kind: "pattern", Line: 1})
	}
	p.dedupe()
	return p
}

func runtimeTool(s string) string {
	s = strings.ToLower(s)
	for _, t := range []string{"java", "jdk", "node", "go", "python", "dotnet"} {
		if strings.Contains(s, t) {
			if t == "jdk" {
				return "java"
			}
			return t
		}
	}
	return ""
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' }) {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// matrixCombos expands a job matrix into its combinations, capped at 64.
// Scalar lists multiply; `include` entries are added as they are.
func matrixCombos(m *yaml.Node) []map[string]interface{} {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	combos := []map[string]interface{}{{}}
	var includes []map[string]interface{}
	for _, p := range pairs(m) {
		switch p.Key {
		case "include":
			for _, it := range items(p.Value) {
				var v map[string]interface{}
				if it.Decode(&v) == nil {
					includes = append(includes, v)
				}
			}
		case "exclude":
		default:
			var vals []interface{}
			if p.Value.Decode(&vals) != nil {
				continue
			}
			var next []map[string]interface{}
			for _, c := range combos {
				for _, v := range vals {
					n := map[string]interface{}{}
					for k, x := range c {
						n[k] = x
					}
					n[p.Key] = v
					next = append(next, n)
					if len(next) >= 64 {
						break
					}
				}
			}
			combos = next
		}
	}
	if len(combos) == 1 && len(combos[0]) == 0 {
		combos = nil
	}
	combos = append(combos, includes...)
	return combos
}

// expandGH substitutes ${{ matrix.a.b }} and ${{ env.X }}; any other
// expression is left for NormalizeImage to drop.
func expandGH(s string, combo map[string]interface{}, env map[string]string) string {
	if !strings.Contains(s, "${{") {
		return s
	}
	return ghExpr.ReplaceAllStringFunc(s, func(m string) string {
		expr := ghExpr.FindStringSubmatch(m)[1]
		switch {
		case strings.HasPrefix(expr, "matrix."):
			var cur interface{} = combo
			for _, k := range strings.Split(strings.TrimPrefix(expr, "matrix."), ".") {
				mm, ok := cur.(map[string]interface{})
				if !ok {
					return m
				}
				cur = mm[k]
			}
			if cur == nil {
				return m
			}
			return fmt.Sprint(cur)
		case strings.HasPrefix(expr, "env."):
			if v, ok := env[strings.TrimPrefix(expr, "env.")]; ok {
				return v
			}
		}
		return m
	})
}

func (p *pipelineFact) dedupe() {
	seen := map[string]bool{}
	var envs []envHit
	for _, e := range p.Environments {
		k := e.Kind + "|" + e.Name
		if !seen[k] {
			seen[k] = true
			envs = append(envs, e)
		}
	}
	p.Environments = envs
	seenD := map[string]bool{}
	var ds []delegation
	for _, d := range p.Delegates {
		k := d.Target + "@" + d.Ref
		if !seenD[k] {
			seenD[k] = true
			ds = append(ds, d)
		}
	}
	p.Delegates = ds
	sort.Strings(p.Calls)
}

// ---------------------------------------------------------------------------
// Jenkins, declarative subset
// ---------------------------------------------------------------------------

var (
	jenkinsLibrary  = regexp.MustCompile(`@Library\s*\(\s*\[?\s*['"]([^'"]+)['"]|\blibrary\s*\(?\s*(?:identifier:\s*)?['"]([^'"]+)['"]`)
	jenkinsStage    = regexp.MustCompile(`\bstage\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	jenkinsShell    = regexp.MustCompile(`(?s)\b(?:sh|bat|powershell)\s*\(?\s*(?:script:\s*)?(?:'''(.*?)'''|"""(.*?)"""|'([^'\n]*)'|"([^"\n]*)")`)
	jenkinsDeclared = regexp.MustCompile(`(?m)^\s*pipeline\s*\{`)
	jenkinsDockerB  = regexp.MustCompile(`docker\.build\s*\(\s*["']([^"']+)["'](?:\s*,\s*["']([^"']*)["'])?`)
	groovyComment   = regexp.MustCompile(`(?m)^\s*//[^\n]*|/\*(?s:.*?)\*/`)
)

func readJenkinsfile(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "jenkins")
	src := string(content)
	src = groovyComment.ReplaceAllStringFunc(src, func(m string) string { return strings.Repeat("\n", strings.Count(m, "\n")) })
	if !jenkinsDeclared.MatchString(src) {
		p.Parsed = "partial"
	}
	lineAt := func(i int) int { return strings.Count(src[:i], "\n") + 1 }
	for _, m := range jenkinsLibrary.FindAllStringSubmatchIndex(src, -1) {
		lib := ""
		for g := 1; g <= 2; g++ {
			if m[2*g] >= 0 {
				lib = src[m[2*g]:m[2*g+1]]
			}
		}
		name, ref, _ := strings.Cut(lib, "@")
		p.Delegates = append(p.Delegates, delegation{Target: "jenkins-library:" + name, Ref: ref, Line: lineAt(m[0])})
	}
	for _, m := range jenkinsStage.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		p.classifyText(name, lineAt(m[0]))
	}
	for _, m := range jenkinsShell.FindAllStringSubmatchIndex(src, -1) {
		body := ""
		for g := 1; g <= 4; g++ {
			if m[2*g] >= 0 {
				body = src[m[2*g]:m[2*g+1]]
			}
		}
		p.readShell(body, lineAt(m[0]), dirOf(file))
	}
	for _, m := range jenkinsDockerB.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ctx := "."
		args := ""
		if m[4] >= 0 {
			args = src[m[4]:m[5]]
		}
		fields := splitShellArgs(args)
		df := ""
		for i := 0; i < len(fields); i++ {
			if fields[i] == "-f" && i+1 < len(fields) {
				df = fields[i+1]
				i++
			} else if !strings.HasPrefix(fields[i], "-") {
				ctx = fields[i]
			}
		}
		b := &imageBuild{Names: []ImageName{NormalizeImage(name)}, BuiltBy: "pipeline", File: file, Line: lineAt(m[0]), Context: joinPath(dirOf(file), ctx)}
		if df == "" {
			df = stdpath.Join(ctx, "Dockerfile")
		}
		b.Dockerfile = joinPath(dirOf(file), df)
		p.Builds = append(p.Builds, b)
		p.hit("package", b.Line)
	}
	p.classifyText(src, 0)
	// classifyText over the whole file finds tools; stages found only there
	// have no line of their own and are dropped unless a stage or step
	// already placed them.
	for st, l := range p.stages {
		if l == 0 {
			delete(p.stages, st)
		}
	}
	p.dedupe()
	return p
}

// ---------------------------------------------------------------------------
// GitLab CI
// ---------------------------------------------------------------------------

var gitlabReserved = map[string]bool{
	"stages": true, "include": true, "variables": true, "default": true, "workflow": true,
	"image": true, "services": true, "before_script": true, "after_script": true, "cache": true, "pages": false,
}

func readGitLabCI(file string, content []byte) *pipelineFact {
	p := newPipeline(file, "gitlab")
	docs := yamlDocs(content)
	if len(docs) == 0 {
		p.Parsed = "none"
		return p
	}
	doc := docs[0]
	for _, inc := range includeEntries(get(doc, "include")) {
		p.Delegates = append(p.Delegates, inc)
		for _, st := range stageOfTemplate(inc.Target) {
			p.hit(st, inc.Line)
		}
	}
	for _, s := range strs(get(doc, "stages")) {
		p.classifyText(s, line(get(doc, "stages")))
	}
	for _, job := range pairs(doc) {
		if gitlabReserved[job.Key] || strings.HasPrefix(job.Key, ".") || job.Value == nil || job.Value.Kind != yaml.MappingNode {
			continue
		}
		ln := line(job.KeyN)
		p.classifyText(job.Key+" "+str(get(job.Value, "stage")), ln)
		for _, key := range []string{"before_script", "script", "after_script"} {
			for _, s := range strs(get(job.Value, key)) {
				p.readShell(s, ln, ".")
			}
		}
		if env := get(job.Value, "environment"); env != nil {
			name := str(env)
			if name == "" {
				name = str(get(env, "name"))
			}
			switch {
			case name == "":
			case regexp.MustCompile(`\$(CI_COMMIT_REF_SLUG|CI_MERGE_REQUEST_IID|CI_COMMIT_REF_NAME|CI_MERGE_REQUEST)`).MatchString(name) || strings.Contains(name, "review/"):
				p.Environments = append(p.Environments, envHit{Name: "per branch or merge request", Kind: "pattern", Line: line(env)})
			case !strings.Contains(name, "$"):
				p.Environments = append(p.Environments, envHit{Name: name, Kind: "enumerated", Line: line(env)})
			}
		}
		for _, r := range items(get(job.Value, "rules")) {
			p.Paths = append(p.Paths, strs(get(r, "changes"))...)
		}
	}
	p.Triggers = []string{"push", "merge_request"}
	p.dedupe()
	return p
}

func includeEntries(n *yaml.Node) []delegation {
	var out []delegation
	for _, it := range keysOrItemsList(n) {
		switch it.Kind {
		case yaml.ScalarNode:
			v := str(it)
			if strings.HasPrefix(v, "/") || !strings.Contains(v, "://") {
				// A local include is part of this pipeline, not a delegation.
				continue
			}
			out = append(out, delegation{Target: v, Line: line(it)})
		case yaml.MappingNode:
			switch {
			case get(it, "local") != nil:
				continue
			case get(it, "project") != nil:
				for _, f := range strs(get(it, "file")) {
					out = append(out, delegation{Target: str(get(it, "project")) + "/" + strings.TrimPrefix(f, "/"), Ref: str(get(it, "ref")), Line: line(it)})
				}
			case get(it, "remote") != nil:
				out = append(out, delegation{Target: str(get(it, "remote")), Line: line(it)})
			case get(it, "template") != nil:
				out = append(out, delegation{Target: "gitlab-template:" + str(get(it, "template")), Line: line(it)})
			case get(it, "component") != nil:
				target, ref, _ := strings.Cut(str(get(it, "component")), "@")
				out = append(out, delegation{Target: target, Ref: ref, Line: line(it)})
			}
		}
	}
	return out
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

// readOtherCI records a pipeline whose format is recognised but not read.
func readOtherCI(file string, system string) *pipelineFact {
	p := newPipeline(file, system)
	p.Parsed = "none"
	return p
}

func ciSystemOf(file string) string {
	lower := strings.ToLower(file)
	base := stdpath.Base(lower)
	switch {
	case strings.Contains(lower, ".github/workflows/"):
		return "github_actions"
	case strings.HasPrefix(base, "jenkinsfile") || strings.HasSuffix(base, ".jenkinsfile"):
		return "jenkins"
	case base == ".gitlab-ci.yml" || base == ".gitlab-ci.yaml" || strings.Contains(lower, ".gitlab/ci/"):
		return "gitlab"
	case strings.Contains(base, "azure-pipelines") || strings.Contains(lower, ".azure-pipelines/"):
		return "azure_pipelines"
	case strings.Contains(lower, ".circleci/"):
		return "circleci"
	case base == ".travis.yml":
		return "travis"
	case base == "bitbucket-pipelines.yml":
		return "bitbucket"
	case strings.HasPrefix(base, "cloudbuild"):
		return "cloud_build"
	case strings.Contains(lower, ".github/actions/") || base == "action.yml" || base == "action.yaml":
		return "github_action"
	}
	return "other"
}
