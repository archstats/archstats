package cicd

import (
	"fmt"
	stdpath "path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// What a step does is read from what it runs, and only then from its name:
// "Build and push" says less than `docker push`. One step gets one verb, the
// furthest along the delivery it reaches: a step that builds and pushes an
// image publishes it.

var verbRules = []struct {
	verb string
	re   *regexp.Regexp
}{
	{"approve", regexp.MustCompile(`(?i)\binput\s*(\(|message|\{)|manual-?approval|\bapprovals?\b|ManualValidation@|\btype:\s*approval\b|go-?no-?go|change-request`)},
	{"deploy", regexp.MustCompile(`(?i)\bhelm\s+(upgrade|install|rollback)\b|\bkubectl\s+(apply|set\s+image|rollout|create|replace)\b|\brancher\s+(kubectl|apps?\s+(install|upgrade)|app\s+upgrade|catalog)\b|\bkustomize\s+build\b|\bargocd\s+app\b|\bflux\s+(reconcile|create)\b|\bskaffold\s+(run|deploy)\b|\bterraform\s+apply\b|\b(sam|serverless|sls)\s+deploy\b|\bgcloud\s+[a-z ]*deploy\b|\baz\s+(webapp|containerapp|functionapp)\b|webapps-deploy|functions-action|k8s-deploy|deploy-cloudrun|amazon-ecs-deploy|\baws\s+(ecs\s+update-service|lambda\s+update-function|cloudformation\s+deploy)|\bcf\s+push\b|\bfly\s+deploy\b|\bvercel\b.*--prod|\bnetlify\s+deploy\b|\bheroku\b|\bansible-playbook\b|\bdocker[ -]compose\b[^\n]*\bup\b|HelmDeploy@|KubernetesManifest@|AzureWebApp@|AzureFunctionApp@`)},
	{"publish", regexp.MustCompile(`(?i)\bdocker\s+push\b|\bpodman\s+push\b|buildx\b[^\n]*--push|\bpush:\s*true|\btwine\s+upload\b|\bnpm\s+publish\b|\byarn\s+publish\b|\bmvnw?\b[^\n]*\bdeploy\b|\bgradlew?\b[^\n]*\bpublish|\bgh\s+release\s+(create|upload)\b|action-gh-release|upload-release-asset|push-to-registry|\bjf(rog)?\s+rt\s+(u|upload)\b|chart-releaser|\bhelm\s+push\b|\bgoreleaser\b|\bcrane\s+push\b|\bskopeo\s+copy\b|\bdotnet\s+nuget\s+push\b|publishImageConfig`)},
	{"deploy", regexp.MustCompile(`(?i)\bdeploy(s|ment|ing)?\b|\brelease[-_ ]to\b|\brollout\b|\bpromote\b`)},
	{"package", regexp.MustCompile(`(?i)\b(docker|podman)\s+(buildx\s+)?build\b|\bbuildx\b|build-push-action|\bjib\b|bootBuildImage|build-image|\bkaniko\b|\bbuildah\b|\bpack\s+build\b|\bko\s+build\b|\bcompose\s+build\b|\bhelm\s+package\b|\bnpm\s+pack\b|\bsam\s+build\b|\bbake\b|docker\.build\(|Docker@\d|\bimage\.(build|push)\b`)},
	{"scan", regexp.MustCompile(`(?i)codeql|sonar|\btrivy\b|\bsnyk\b|\bgrype\b|anchore|checkmarx|fortify|dependency-check|owasp|semgrep|gitleaks|zizmor|scorecard|nexus-?iq|blackduck|whitesource|veracode|\bsast\b|\bdast\b|static-analysis|dependency-review|license-check|secret-detection|\bnpm\s+audit\b|\bpip-audit\b|cosign\s+verify`)},
	{"test", regexp.MustCompile(`(?i)\btests?\b|\bpytest\b|\bjest\b|\bvitest\b|\bgo\s+test\b|\bmvnw?\b[^\n]*\b(verify|test)\b|\bgradlew?\b[^\n]*\b(test|check)\b|\bnpm\s+(run\s+)?test\b|\byarn\s+test\b|\bpnpm\s+test\b|\bcypress\b|\bplaywright\b|\bjunit\b|\bkarma\b|\bphpunit\b|\brspec\b|\btox\b|\bnox\b|\bdotnet\s+test\b|\bcoverage\b|\be2e\b|integration-tests?|\blint\b|\beslint\b|\bgolangci\b|\bflake8\b|\bruff\b|\bmypy\b`)},
	{"build", regexp.MustCompile(`(?i)\bmvnw?\b|\bmaven\b|\bgradlew?\b|\bnpm\s+(ci|install|i|run\s+build)\b|\byarn(\s+install|\s+build|\s*$)|\bpnpm\s+(install|build|i)\b|\bgo\s+build\b|\bdotnet\s+(build|publish|restore)\b|\bcargo\s+build\b|\bmake\b|\bant\b|\bsbt\b|\bbazel\s+build\b|\btsc\b|\bcomposer\s+install\b|\bpip\s+install\b|\bpoetry\s+install\b|\buv\s+sync\b|\bwails\s+build\b|\btauri\s+build\b|\bwebpack\b|\bvite\s+build\b|\bnuxi?\s+(build|generate)\b|\bcompile\b|\bbuild\b|Maven@\d|Gradle@\d|DotNetCoreCLI@\d|Npm@\d`)},
	{"checkout", regexp.MustCompile(`(?i)actions/checkout|\bcheckout\s+scm\b|\bgit\s+(clone|checkout|fetch)\b|^\s*checkout\s*$|\bcheckout\b`)},
	{"setup", regexp.MustCompile(`(?i)setup-[a-z]+|actions/cache|\bcache\b|\bdocker\s+login\b|login-action|\brancher\s+(login|context)\b|configure-aws-credentials|google-github-actions/auth|azure/login|\bnvm\s+use\b|\bsdk\s+use\b|\btool\s*\(|\binstall\b|\bwithCredentials\b|\bartifact\b`)},
}

// classifyVerb reads a step. sure is false when only the loose rules (a
// "deploy" in a name, a bare "build") matched.
func classifyVerb(text string) (string, bool) {
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	for i, r := range verbRules {
		if r.re.MatchString(text) {
			// The second deploy rule and the bare words in the last three
			// are names, not commands.
			return r.verb, i != 3
		}
	}
	return "", false
}

var toolOrder = []struct {
	tool string
	re   *regexp.Regexp
}{
	{"rancher", regexp.MustCompile(`(?i)\brancher\b`)},
	{"helm", regexp.MustCompile(`(?i)\bhelm\b|setup-helm|HelmDeploy@|HelmInstaller@`)},
	{"kubectl", regexp.MustCompile(`(?i)\bkubectl\b|k8s-deploy|setup-kubectl|KubernetesManifest@|Kubernetes@`)},
	{"kustomize", regexp.MustCompile(`(?i)\bkustomize\b`)},
	{"skaffold", regexp.MustCompile(`(?i)\bskaffold\b`)},
	{"argocd", regexp.MustCompile(`(?i)\bargocd\b`)},
	{"terraform", regexp.MustCompile(`(?i)\bterraform\b|setup-terraform|\btofu\b|TerraformTask`)},
	{"jib", regexp.MustCompile(`(?i)\bjib\b`)},
	{"docker", regexp.MustCompile(`(?i)\bdocker\b|\bpodman\b|buildx|build-push-action|login-action|\bkaniko\b|\bbuildah\b|Docker@\d|\bimage\.(build|push)\b`)},
	{"maven", regexp.MustCompile(`(?i)\bmvnw?\b|setup-java[^\n]*maven|\bmaven\b|Maven@\d`)},
	{"gradle", regexp.MustCompile(`(?i)\bgradlew?\b|gradle-build-action|setup-gradle|Gradle@\d`)},
	{"npm", regexp.MustCompile(`(?i)\bnpm\b|\byarn\b|\bpnpm\b|setup-node|\bnpx\b|Npm@\d`)},
	{"go", regexp.MustCompile(`(?i)\bgo\s+(build|test|vet|mod|run|install)\b|setup-go|golangci`)},
	{"dotnet", regexp.MustCompile(`(?i)\bdotnet\b|setup-dotnet|DotNetCoreCLI@|NuGet`)},
	{"python", regexp.MustCompile(`(?i)\bpip\b|\bpoetry\b|\bpytest\b|setup-python|\buv\s+(sync|run)\b|\btox\b|\bpython3?\b`)},
	{"sam", regexp.MustCompile(`(?i)\bsam\s+(build|deploy)\b|setup-sam`)},
	{"serverless", regexp.MustCompile(`(?i)\bserverless\b|\bsls\b`)},
	{"gcloud", regexp.MustCompile(`(?i)\bgcloud\b|google-github-actions`)},
	{"az", regexp.MustCompile(`(?i)\baz\s+[a-z]+|azure/|Azure[A-Za-z]+@`)},
	{"aws", regexp.MustCompile(`(?i)\baws\s+[a-z]+|aws-actions/`)},
	{"codeql", regexp.MustCompile(`(?i)codeql`)},
	{"sonar", regexp.MustCompile(`(?i)sonar`)},
	{"trivy", regexp.MustCompile(`(?i)\btrivy\b`)},
	{"ansible", regexp.MustCompile(`(?i)ansible`)},
	{"git", regexp.MustCompile(`(?i)actions/checkout|\bgit\b|checkout\s+scm`)},
	{"wails", regexp.MustCompile(`(?i)\bwails\b`)},
	{"make", regexp.MustCompile(`(?i)\bmake\b`)},
	{"gh", regexp.MustCompile(`(?i)\bgh\s+[a-z]+`)},
	{"curl", regexp.MustCompile(`(?i)\bcurl\b|\bwget\b`)},
}

// classifyTool names the first tool a piece of step text runs.
func classifyTool(text string) string {
	for _, t := range toolOrder {
		if t.re.MatchString(text) {
			return t.tool
		}
	}
	return ""
}

var (
	camelWords  = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	notifyWords = regexp.MustCompile(`\b(notify|notification|notifications|slack|mail|email|teams|report)\b`)
)

// A reusable template is classified by its name: build-maven.yml builds,
// deployment-nonproduction.yml deploys, codeQL.yml scans.
func verbOfTemplate(name string) string {
	n := camelWords.ReplaceAllString(stdpath.Base(name), "$1 $2")
	n = strings.ToLower(n)
	words := strings.NewReplacer("-", " ", "_", " ", ".yml", "", ".yaml", "", ".groovy", "").Replace(n)
	if notifyWords.MatchString(words) {
		return "other"
	}
	if v, _ := classifyVerb(words); v != "" {
		return v
	}
	return "other"
}

// ---------------------------------------------------------------------------
// Commands inside a step
// ---------------------------------------------------------------------------

var (
	dockerBuildCmd  = regexp.MustCompile(`(?m)\b(?:docker|podman)\s+(?:buildx\s+)?build\s+([^\n;&|]+)`)
	dockerPushCmd   = regexp.MustCompile(`(?m)\b(?:docker|podman)\s+push\s+(\S+)`)
	helmDeployCmd   = regexp.MustCompile(`(?m)\bhelm\s+(?:upgrade\s+--install|upgrade|install)\s+([^\n;&|]+)`)
	kubectlApplyCmd = regexp.MustCompile(`(?m)\b(rancher\s+)?kubectl\s+(?:[^\n;&|]*?\s)?(?:apply|create|replace)\s+([^\n;&|]+)`)
	kubectlImageCmd = regexp.MustCompile(`(?m)\b(rancher\s+)?kubectl\s+(?:[^\n;&|]*?\s)?set\s+image\s+([^\n;&|]+)`)
	kustomizeCmd    = regexp.MustCompile(`(?m)\bkustomize\s+build\s+(\S+)`)
	cloudRunCmd     = regexp.MustCompile(`(?m)\bgcloud\s+(?:beta\s+)?run\s+(?:deploy|jobs\s+(?:deploy|update))\s+([A-Za-z0-9_$.{}-]+)([^\n;&|]*)`)
	lambdaCmd       = regexp.MustCompile(`(?m)--function-name[ =](\S+)`)
	ecsCmd          = regexp.MustCompile(`(?m)\baws\s+ecs\s+update-service[^\n]*--service[ =](\S+)`)
	composeCmd      = regexp.MustCompile(`(?m)\bdocker[ -]compose\b[^\n]*\b(build|up)\b`)
	skaffoldCmd     = regexp.MustCompile(`(?m)\bskaffold\s+(run|deploy|build)\b`)
	samCmd          = regexp.MustCompile(`(?m)\b(sam|serverless|sls)\s+deploy\b`)
	jibCmd          = regexp.MustCompile(`(?m)\bjib(:build|:dockerBuild|Build|DockerBuild)?\b`)
	buildToolCmd    = regexp.MustCompile(`(?m)\b(mvnw?|gradlew?|\./mvnw|\./gradlew)\b[^\n]*\b(package|install|verify|build|bootJar|assemble|jib)\b|\bdotnet\s+(publish|build)\b|\bnpm\s+run\s+build\b|\bgo\s+build\b|\bwails\s+build\b|\btauri\s+build\b|\belectron-builder\b`)
	rancherLoginCmd = regexp.MustCompile(`(?m)\brancher\s+login\s+([^\n;&|]+)`)
	rancherCtxCmd   = regexp.MustCompile(`(?m)\brancher\s+context\s+switch\s+(\S+)`)
	rancherAppCmd   = regexp.MustCompile(`(?m)\brancher\s+apps?\s+(?:install|upgrade)\s+([^\n;&|]+)`)
)

// readShell reads a script for builds, deploys and build tools, in the
// current job and step. dir is the directory commands run in, for resolving
// relative paths.
func (p *pipelineFact) readShell(script string, line int, dir string) {
	script = joinContinuations(script)
	// An expression is one argument however it is spaced:
	// `pr-${{ github.event.number }}` must not split into three.
	script = ghExpr.ReplaceAllStringFunc(script, func(m string) string { return strings.ReplaceAll(m, " ", "") })
	p.scriptFlags(script, line)
	if rt := runtimeOfScript(script); rt.Tool != "" {
		rt.Line = line
		p.Runtimes = append(p.Runtimes, rt)
	}
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
					b.Names = append(b.Names, NormalizeImage(p.expand(args[i+1])))
					b.RawTags = append(b.RawTags, args[i+1])
					i++
				}
			case strings.HasPrefix(a, "--tag="), strings.HasPrefix(a, "-t="):
				v := a[strings.Index(a, "=")+1:]
				b.Names = append(b.Names, NormalizeImage(p.expand(v)))
				b.RawTags = append(b.RawTags, v)
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
		}
		b.Dockerfile = joinPath(dir, df)
		p.addBuild(b)
	}
	for _, m := range dockerPushCmd.FindAllStringSubmatch(script, -1) {
		if p.cs != nil && p.cs.Target == "" {
			if n := NormalizeImage(p.expand(m[1])); n.Repo != "" {
				p.cs.Target = n.String()
			}
		}
		p.pushed = append(p.pushed, pushHit{Raw: m[1], Line: line, Job: p.jobID()})
	}
	for _, m := range helmDeployCmd.FindAllStringSubmatch(script, -1) {
		args := splitShellArgs(m[1])
		var pos []string
		for i := 0; i < len(args); i++ {
			if strings.HasPrefix(args[i], "-") {
				if !strings.Contains(args[i], "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && args[i] != "--install" && args[i] != "--wait" && args[i] != "--atomic" && args[i] != "--create-namespace" && args[i] != "--debug" {
					if args[i] == "-f" || args[i] == "--values" {
						p.addDeploy(&deployHit{Kind: "helm_values", Target: joinPath(dir, args[i+1]), Line: line})
					}
					i++
				}
				continue
			}
			pos = append(pos, args[i])
		}
		if len(pos) >= 2 {
			p.addDeploy(&deployHit{Kind: "helm", Target: joinPath(dir, pos[1]), Line: line})
		}
	}
	server, context := p.rancherSession(script)
	for _, m := range kubectlApplyCmd.FindAllStringSubmatch(script, -1) {
		args := splitShellArgs(m[2])
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "-f" || args[i] == "--filename" || args[i] == "-k" || args[i] == "--kustomize" {
				d := &deployHit{Kind: "kubectl", Target: joinPath(dir, args[i+1]), Line: line}
				if m[1] != "" {
					d.Kind, d.Server, d.Context = "rancher", server, context
				}
				p.addDeploy(d)
			}
		}
	}
	for _, m := range kubectlImageCmd.FindAllStringSubmatch(script, -1) {
		d := &deployHit{Kind: "kubectl", Target: firstPositional(m[2]), Line: line}
		if m[1] != "" {
			d.Kind, d.Server, d.Context = "rancher", server, context
		}
		// container=image: the image it now runs.
		for _, a := range splitShellArgs(m[2]) {
			if _, img, ok := strings.Cut(a, "="); ok && !strings.HasPrefix(a, "-") {
				d.Image = NormalizeImage(p.expand(img))
			}
		}
		p.addDeploy(d)
	}
	for _, m := range rancherAppCmd.FindAllStringSubmatch(script, -1) {
		p.addDeploy(&deployHit{Kind: "rancher", Target: firstPositional(m[1]), Server: server, Context: context, Line: line})
	}
	for _, m := range kustomizeCmd.FindAllStringSubmatch(script, -1) {
		p.addDeploy(&deployHit{Kind: "kustomize", Target: joinPath(dir, m[1]), Line: line})
	}
	for _, m := range cloudRunCmd.FindAllStringSubmatch(script, -1) {
		p.addDeploy(&deployHit{Kind: "cloud", Target: m[1], Line: line})
	}
	for _, m := range lambdaCmd.FindAllStringSubmatch(script, -1) {
		p.addDeploy(&deployHit{Kind: "cloud", Target: m[1], Line: line})
	}
	for _, m := range ecsCmd.FindAllStringSubmatch(script, -1) {
		p.addDeploy(&deployHit{Kind: "cloud", Target: m[1], Line: line})
	}
	for _, m := range composeCmd.FindAllStringSubmatch(script, -1) {
		if m[1] == "up" {
			p.addDeploy(&deployHit{Kind: "compose", Line: line})
		}
	}
	for _, m := range skaffoldCmd.FindAllStringSubmatch(script, -1) {
		if m[1] != "deploy" {
			// skaffold build and skaffold run build every artifact.
			p.addBuild(&imageBuild{BuiltBy: "skaffold", File: p.File, Line: line})
		}
		if m[1] != "build" {
			p.addDeploy(&deployHit{Kind: "skaffold", Line: line})
		}
	}
	if samCmd.MatchString(script) {
		p.addDeploy(&deployHit{Kind: "sam", Line: line})
	}
	if jibCmd.MatchString(script) && (strings.Contains(script, "mvn") || strings.Contains(script, "gradle")) {
		p.addBuild(&imageBuild{BuiltBy: "jib", File: p.File, Line: line})
	}
	if buildToolCmd.MatchString(script) {
		p.buildLocal = true
		if !p.BuildTool {
			p.BuildTool, p.BuildToolLine, p.buildToolJob = true, line, p.jobID()
		}
	}
}

type pushHit struct {
	Raw  string
	Line int
	Job  string
}

func (p *pipelineFact) jobID() string {
	if p.cj == nil {
		return ""
	}
	return p.cj.ID
}

// rancherSession reads the `rancher login` and `rancher context switch` in a
// script, falling back to the last ones this job saw: GitLab jobs log in in
// before_script and deploy in script.
func (p *pipelineFact) rancherSession(script string) (server, context string) {
	for _, m := range rancherLoginCmd.FindAllStringSubmatch(script, -1) {
		args := splitShellArgs(m[1])
		for i := 0; i < len(args); i++ {
			switch {
			case args[i] == "--context" && i+1 < len(args):
				context = args[i+1]
				i++
			case strings.HasPrefix(args[i], "--context="):
				context = strings.TrimPrefix(args[i], "--context=")
			case (args[i] == "--token" || args[i] == "-t" || args[i] == "--name" || args[i] == "--cacert") && i+1 < len(args):
				i++
			case strings.HasPrefix(args[i], "-"):
			case server == "":
				server = args[i]
			}
		}
	}
	for _, m := range rancherCtxCmd.FindAllStringSubmatch(script, -1) {
		context = m[1]
	}
	if p.cj != nil {
		if server == "" {
			server = p.rancherServer[p.cj.ID]
		}
		if context == "" {
			context = p.rancherContext[p.cj.ID]
		}
		if p.rancherServer == nil {
			p.rancherServer, p.rancherContext = map[string]string{}, map[string]string{}
		}
		p.rancherServer[p.cj.ID], p.rancherContext[p.cj.ID] = server, context
	}
	return server, context
}

func firstPositional(s string) string {
	for _, a := range splitShellArgs(s) {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

var scriptRuntime = regexp.MustCompile(`(?i)\b(?:nvm\s+(?:install|use)|sdk\s+(?:install|use)\s+java)\s+(\d+(?:\.\d+)?)`)

func runtimeOfScript(script string) runtimeHit {
	m := scriptRuntime.FindStringSubmatch(script)
	if m == nil {
		return runtimeHit{}
	}
	if strings.Contains(strings.ToLower(m[0]), "java") {
		return runtimeHit{Tool: "java", Version: m[1]}
	}
	return runtimeHit{Tool: "node", Version: m[1]}
}

var shellVar = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// expand substitutes variables the file itself defines, so an image named
// `$IMAGE_NAME:$TAG` joins when IMAGE_NAME is set in the file.
func (p *pipelineFact) expand(s string) string {
	if !strings.Contains(s, "$") || len(p.vars) == 0 {
		return s
	}
	for i := 0; i < 3 && strings.Contains(s, "$"); i++ {
		s = shellVar.ReplaceAllStringFunc(s, func(m string) string {
			sub := shellVar.FindStringSubmatch(m)
			k := sub[1] + sub[2]
			if v, ok := p.vars[k]; ok {
				return v
			}
			return m
		})
	}
	return s
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

var ghExpr = regexp.MustCompile(`\$\{\{\s*([^}]+?)\s*\}\}`)

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

// localUses spells a same-repository reference the one way: `$/.github/...`
// names this repository as `./.github/...` does.
func localUses(uses string) string {
	if strings.HasPrefix(uses, "$/") {
		return "./" + strings.TrimPrefix(uses, "$/")
	}
	return uses
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
