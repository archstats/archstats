package deployables

import (
	stdpath "path"
	"strings"

	"gopkg.in/yaml.v3"
)

// joinPath resolves a path written in a file against that file's directory,
// clean and repository-relative. A path that climbs out of the workspace is
// returned as "".
func joinPath(dir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return dir
	}
	if strings.HasPrefix(p, "/") {
		// Absolute paths in a repository file are not about this checkout.
		return ""
	}
	j := stdpath.Clean(stdpath.Join(dir, p))
	if j == ".." || strings.HasPrefix(j, "../") {
		return ""
	}
	return j
}

func dirOf(p string) string {
	d := stdpath.Dir(p)
	if d == "" {
		return "."
	}
	return d
}

// ---------------------------------------------------------------------------
// docker compose
// ---------------------------------------------------------------------------

// readCompose reads services from a compose file after interpolating it with
// the .env beside it, as compose itself does.
func readCompose(file string, content []byte, vars map[string]string) ([]*workload, []*imageBuild) {
	docs := yamlDocs([]byte(interpolate(string(content), vars)))
	if len(docs) == 0 {
		return nil, nil
	}
	services := get(docs[0], "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil, nil
	}
	dir := dirOf(file)
	var workloads []*workload
	var builds []*imageBuild
	for _, p := range pairs(services) {
		svc := p.Value
		if svc == nil || svc.Kind != yaml.MappingNode {
			continue
		}
		w := &workload{
			Key:  file + "#" + p.Key,
			Name: p.Key,
			Kind: "compose_service",
			File: file,
			Line: line(p.KeyN),
		}
		for _, alias := range []string{str(get(svc, "container_name")), str(get(svc, "hostname"))} {
			if alias != "" && alias != p.Key {
				w.Aliases = append(w.Aliases, alias)
			}
		}
		image := str(get(svc, "image"))
		if image != "" {
			w.Images = append(w.Images, imageRef{Raw: image, Name: NormalizeImage(image), Workload: w.Key, File: file, Line: line(get(svc, "image")), Source: "compose"})
		}
		if b := get(svc, "build"); b != nil {
			ib := &imageBuild{BuiltBy: "compose", File: file, Line: line(b), Aliases: []string{p.Key}, Workload: w.Key}
			switch b.Kind {
			case yaml.ScalarNode:
				ib.Context = joinPath(dir, str(b))
			case yaml.MappingNode:
				ib.Context = joinPath(dir, str(get(b, "context")))
				ib.Target = str(get(b, "target"))
				if df := str(get(b, "dockerfile")); df != "" {
					ib.Dockerfile = joinPath(ib.Context, df)
				}
				if get(b, "dockerfile_inline") != nil {
					ib.Dockerfile = ""
				}
			}
			if ctx := str(get(b, "context")); strings.Contains(ctx, "://") || strings.HasPrefix(ctx, "git@") {
				// A remote build context builds nothing from this workspace.
				ib = nil
			}
			if ib != nil && ib.Context != "" {
				if ib.Dockerfile == "" && get(b, "dockerfile_inline") == nil {
					ib.Dockerfile = joinPath(ib.Context, "Dockerfile")
				}
				if image != "" {
					ib.Names = append(ib.Names, NormalizeImage(image))
					ib.Declared = true
				}
				w.Build = ib
				builds = append(builds, ib)
			}
		}
		for _, e := range envEntries(get(svc, "environment")) {
			// `- CART_ADDR` with no value passes the variable through from
			// where compose runs, which includes the .env beside the file.
			if e.Value == "" {
				if v, ok := vars[e.Key]; ok {
					e.Value = v
				}
			}
			w.Env = append(w.Env, configEntry{Key: e.Key, Value: e.Value, File: file, Line: e.Line})
		}
		for _, ef := range items(get(svc, "env_file")) {
			pth := str(ef)
			if pth == "" {
				pth = str(get(ef, "path"))
			}
			if j := joinPath(dir, pth); j != "" {
				w.EnvFiles = append(w.EnvFiles, j)
			}
		}
		if ef := get(svc, "env_file"); ef != nil && ef.Kind == yaml.ScalarNode {
			if j := joinPath(dir, str(ef)); j != "" {
				w.EnvFiles = append(w.EnvFiles, j)
			}
		}
		for _, d := range keysOrItems(get(svc, "depends_on")) {
			if name := str(d); name != "" {
				w.DependsOn = append(w.DependsOn, keyValue{Key: name, Line: line(d)})
			}
		}
		workloads = append(workloads, w)
	}
	return workloads, builds
}

// ---------------------------------------------------------------------------
// Skaffold
// ---------------------------------------------------------------------------

func readSkaffold(file string, content []byte) []*imageBuild {
	var out []*imageBuild
	dir := dirOf(file)
	for _, doc := range yamlDocs(content) {
		if !strings.HasPrefix(str(get(doc, "apiVersion")), "skaffold/") {
			continue
		}
		lists := []*yaml.Node{at(doc, "build", "artifacts")}
		for _, prof := range items(get(doc, "profiles")) {
			lists = append(lists, at(prof, "build", "artifacts"))
		}
		seen := map[string]bool{}
		for _, list := range lists {
			for _, a := range items(list) {
				image := str(get(a, "image"))
				if image == "" {
					continue
				}
				ctx := joinPath(dir, str(get(a, "context")))
				if ctx == "" {
					continue
				}
				key := image + "|" + ctx
				if seen[key] {
					continue
				}
				seen[key] = true
				b := &imageBuild{Names: []ImageName{NormalizeImage(image)}, Context: ctx, File: file, Line: line(get(a, "image"))}
				switch {
				case get(a, "jib") != nil:
					b.BuiltBy = "jib"
					b.Module = str(at(a, "jib", "project"))
				case get(a, "buildpacks") != nil:
					b.BuiltBy = "buildpacks"
				case get(a, "ko") != nil:
					b.BuiltBy = "ko"
				case get(a, "bazel") != nil:
					b.BuiltBy = "bazel"
				default:
					b.BuiltBy = "skaffold"
					df := str(at(a, "docker", "dockerfile"))
					if df == "" {
						df = "Dockerfile"
					}
					b.Dockerfile = joinPath(ctx, df)
					b.Target = str(at(a, "docker", "target"))
				}
				out = append(out, b)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Kubernetes, kustomize, Argo CD
// ---------------------------------------------------------------------------

var workloadKinds = map[string][]string{
	"Deployment":                  {"spec", "template"},
	"StatefulSet":                 {"spec", "template"},
	"DaemonSet":                   {"spec", "template"},
	"ReplicaSet":                  {"spec", "template"},
	"Job":                         {"spec", "template"},
	"Rollout":                     {"spec", "template"},
	"DeploymentConfig":            {"spec", "template"},
	"CronJob":                     {"spec", "jobTemplate", "spec", "template"},
	"Pod":                         {},
	"Service/serving.knative.dev": {"spec", "template"},
}

type k8sFile struct {
	Workloads  []*workload
	Services   []*k8sService
	ConfigMaps []*configMap
	Argo       []*argoApp
}

func readKubernetes(file string, content []byte) *k8sFile {
	out := &k8sFile{}
	for _, doc := range yamlDocs(content) {
		kind := str(get(doc, "kind"))
		api := str(get(doc, "apiVersion"))
		name := str(at(doc, "metadata", "name"))
		ns := str(at(doc, "metadata", "namespace"))
		if kind == "" {
			continue
		}
		if kind == "List" {
			// A kubectl List wraps its objects.
			for _, it := range items(get(doc, "items")) {
				b, _ := yaml.Marshal(it)
				sub := readKubernetes(file, b)
				out.Workloads = append(out.Workloads, sub.Workloads...)
				out.Services = append(out.Services, sub.Services...)
				out.ConfigMaps = append(out.ConfigMaps, sub.ConfigMaps...)
			}
			continue
		}
		lookupKind := kind
		if kind == "Service" && strings.HasPrefix(api, "serving.knative.dev") {
			lookupKind = "Service/serving.knative.dev"
		}
		if tmplPath, ok := workloadKinds[lookupKind]; ok {
			tmpl := doc
			if len(tmplPath) > 0 {
				tmpl = at(doc, tmplPath...)
			}
			w := &workload{Key: file + "#" + kind + "/" + name, Name: name, Kind: kind, File: file, Line: line(get(doc, "kind")), Namespace: ns, Labels: map[string]string{}}
			for _, p := range pairs(at(tmpl, "metadata", "labels")) {
				w.Labels[p.Key] = str(p.Value)
			}
			spec := get(tmpl, "spec")
			for _, list := range []string{"containers", "initContainers"} {
				for _, c := range items(get(spec, list)) {
					if img := str(get(c, "image")); img != "" {
						w.Images = append(w.Images, imageRef{Raw: img, Name: NormalizeImage(img), Workload: w.Key, File: file, Line: line(get(c, "image")), Source: "k8s"})
					}
					for _, e := range items(get(c, "env")) {
						k := str(get(e, "name"))
						if k == "" {
							continue
						}
						if v := get(e, "value"); v != nil {
							w.Env = append(w.Env, configEntry{Key: k, Value: str(v), File: file, Line: line(e)})
						}
						if cm := str(at(e, "valueFrom", "configMapKeyRef", "name")); cm != "" {
							w.EnvFrom = append(w.EnvFrom, cm+"#"+str(at(e, "valueFrom", "configMapKeyRef", "key"))+"#"+k)
						}
					}
					for _, ef := range items(get(c, "envFrom")) {
						if cm := str(at(ef, "configMapRef", "name")); cm != "" {
							w.EnvFrom = append(w.EnvFrom, cm)
						}
					}
				}
			}
			out.Workloads = append(out.Workloads, w)
			continue
		}
		switch kind {
		case "Service":
			s := &k8sService{Name: name, Namespace: ns, Selector: map[string]string{}, File: file, Line: line(get(doc, "kind"))}
			for _, p := range pairs(at(doc, "spec", "selector")) {
				s.Selector[p.Key] = str(p.Value)
			}
			out.Services = append(out.Services, s)
		case "ConfigMap":
			cm := &configMap{Name: name}
			for _, p := range pairs(get(doc, "data")) {
				cm.Data = append(cm.Data, configEntry{Key: p.Key, Value: str(p.Value), File: file, Line: line(p.KeyN)})
			}
			out.ConfigMaps = append(out.ConfigMaps, cm)
		case "Application":
			if strings.HasPrefix(api, "argoproj.io") {
				out.Argo = append(out.Argo, readArgoSource(name, get(doc, "spec"), file, line(get(doc, "kind"))))
			}
		case "ApplicationSet":
			if strings.HasPrefix(api, "argoproj.io") {
				app := readArgoSource(name, at(doc, "spec", "template", "spec"), file, line(get(doc, "kind")))
				for _, g := range items(at(doc, "spec", "generators")) {
					for _, gp := range pairs(g) {
						switch gp.Key {
						case "list":
							for _, el := range items(get(gp.Value, "elements")) {
								for _, k := range []string{"env", "environment", "stage", "name", "cluster"} {
									if v := str(get(el, k)); v != "" {
										app.Elements = append(app.Elements, v)
										break
									}
								}
							}
						case "pullRequest":
							app.Patterns = append(app.Patterns, "per pull request")
						case "clusters":
							app.Patterns = append(app.Patterns, "per cluster")
						case "git":
							app.Patterns = append(app.Patterns, "per folder or file in git")
						case "scmProvider":
							app.Patterns = append(app.Patterns, "per repository")
						case "matrix", "merge":
							app.Patterns = append(app.Patterns, "per combination of generators")
						}
					}
				}
				out.Argo = append(out.Argo, app)
			}
		}
	}
	return out
}

func readArgoSource(name string, spec *yaml.Node, file string, ln int) *argoApp {
	src := get(spec, "source")
	if src == nil {
		if srcs := items(get(spec, "sources")); len(srcs) > 0 {
			src = srcs[0]
		}
	}
	return &argoApp{Name: name, Path: str(get(src, "path")), RepoURL: str(get(src, "repoURL")), File: file, Line: ln}
}

func readKustomization(file string, content []byte) *kustomization {
	docs := yamlDocs(content)
	if len(docs) == 0 {
		return nil
	}
	doc := docs[0]
	dir := dirOf(file)
	k := &kustomization{Dir: dir, File: file, ConfigMap: map[string][]configEntry{}}
	for _, key := range []string{"resources", "bases", "components"} {
		for _, r := range strs(get(doc, key)) {
			if strings.Contains(r, "://") || strings.HasPrefix(r, "github.com/") {
				continue
			}
			if j := joinPath(dir, r); j != "" {
				k.Resources = append(k.Resources, j)
			}
		}
	}
	for _, im := range items(get(doc, "images")) {
		k.Images = append(k.Images, kustomizeImage{Name: str(get(im, "name")), NewName: str(get(im, "newName")), NewTag: str(get(im, "newTag")), Line: line(im)})
	}
	for _, g := range items(get(doc, "configMapGenerator")) {
		name := str(get(g, "name"))
		for _, lit := range items(get(g, "literals")) {
			kk, v, _ := strings.Cut(str(lit), "=")
			k.ConfigMap[name] = append(k.ConfigMap[name], configEntry{Key: kk, Value: v, File: file, Line: line(lit)})
		}
	}
	return k
}

// ---------------------------------------------------------------------------
// Helm
// ---------------------------------------------------------------------------

func readChart(file string, content []byte) *chart {
	docs := yamlDocs(content)
	if len(docs) == 0 {
		return nil
	}
	c := &chart{Dir: dirOf(file), File: file, Name: str(get(docs[0], "name"))}
	for _, d := range items(get(docs[0], "dependencies")) {
		dep := chartDependency{Name: str(get(d, "name")), Repository: str(get(d, "repository")), Line: line(d)}
		if strings.HasPrefix(dep.Repository, "file://") {
			dep.LocalDir = joinPath(c.Dir, strings.TrimPrefix(dep.Repository, "file://"))
		}
		c.Dependencies = append(c.Dependencies, dep)
	}
	return c
}

// readValues reads a Helm values file (or a values-shaped file such as the
// client's helm-release/prod.yaml) for the images it runs and every setting
// in it, flattened.
func readValues(file string, content []byte) *valuesFile {
	docs := yamlDocs(content)
	v := &valuesFile{File: file, Dir: dirOf(file)}
	if len(docs) == 0 {
		return v
	}
	root := docs[0]
	v.Flat = flatten(root, "")
	var walk func(n *yaml.Node, key string)
	walk = func(n *yaml.Node, key string) {
		n = resolve(n)
		if n == nil {
			return
		}
		switch n.Kind {
		case yaml.MappingNode:
			if repo := get(n, "repository"); repo != nil && repo.Kind == yaml.ScalarNode {
				r := str(repo)
				if r != "" && !strings.Contains(r, "://") && (key == "image" || strings.HasSuffix(strings.ToLower(key), "image") || get(n, "tag") != nil) {
					ref := r
					if reg := str(get(n, "registry")); reg != "" {
						ref = reg + "/" + r
					}
					if tag := str(get(n, "tag")); tag != "" {
						ref += ":" + tag
					}
					v.Images = append(v.Images, imageRef{Raw: ref, Name: NormalizeImage(ref), File: file, Line: line(repo), Source: "helm_values"})
				}
			}
			for _, p := range pairs(n) {
				if (p.Key == "image" || strings.HasSuffix(p.Key, "Image")) && p.Value != nil && p.Value.Kind == yaml.ScalarNode {
					if s := str(p.Value); looksLikeImage(s) {
						v.Images = append(v.Images, imageRef{Raw: s, Name: NormalizeImage(s), File: file, Line: line(p.Value), Source: "helm_values"})
					}
					continue
				}
				walk(p.Value, p.Key)
			}
		case yaml.SequenceNode:
			for _, c := range n.Content {
				walk(c, key)
			}
		}
	}
	walk(root, "")
	return v
}

// ---------------------------------------------------------------------------
// Serverless: SAM, CloudFormation Lambda, Serverless Framework
// ---------------------------------------------------------------------------

func readSAM(file string, content []byte) []*function {
	docs := yamlDocs(content)
	if len(docs) == 0 {
		return nil
	}
	doc := docs[0]
	dir := dirOf(file)
	globalCode := str(at(doc, "Globals", "Function", "CodeUri"))
	globalRuntime := str(at(doc, "Globals", "Function", "Runtime"))
	var out []*function
	for _, p := range pairs(get(doc, "Resources")) {
		typ := str(get(p.Value, "Type"))
		if typ != "AWS::Serverless::Function" && typ != "AWS::Lambda::Function" {
			continue
		}
		props := get(p.Value, "Properties")
		code := str(get(props, "CodeUri"))
		if code == "" && typ == "AWS::Lambda::Function" {
			code = str(get(props, "Code"))
		}
		if code == "" {
			code = globalCode
		}
		if code == "" || strings.HasPrefix(code, "s3://") {
			continue
		}
		cd := joinPath(dir, code)
		if cd == "" {
			continue
		}
		rt := str(get(props, "Runtime"))
		if rt == "" {
			rt = globalRuntime
		}
		out = append(out, &function{Name: p.Key, CodeDir: cd, Handler: str(get(props, "Handler")), Runtime: rt, File: file, Line: line(p.KeyN), BuiltBy: "sam"})
	}
	return out
}

func readServerless(file string, content []byte) []*function {
	docs := yamlDocs(content)
	if len(docs) == 0 {
		return nil
	}
	doc := docs[0]
	dir := dirOf(file)
	runtime := str(at(doc, "provider", "runtime"))
	var out []*function
	for _, p := range pairs(get(doc, "functions")) {
		handler := str(get(p.Value, "handler"))
		if handler == "" {
			continue
		}
		// src/handlers/cart.main: the file is src/handlers/cart, the code
		// lives in its folder.
		handlerFile := handler
		if i := strings.LastIndex(handlerFile, "."); i > strings.LastIndex(handlerFile, "/") {
			handlerFile = handlerFile[:i]
		}
		cd := joinPath(dir, dirOf(handlerFile))
		if cd == "" {
			continue
		}
		rt := str(get(p.Value, "runtime"))
		if rt == "" {
			rt = runtime
		}
		out = append(out, &function{Name: p.Key, CodeDir: cd, Handler: handler, Runtime: rt, File: file, Line: line(p.KeyN), BuiltBy: "serverless"})
	}
	return out
}

// looksLikeImage accepts `nginx`, `ghcr.io/x/api:1.2` and `repo/app`, and
// refuses prose, booleans and pull policies that sit under an image key.
func looksLikeImage(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t") {
		return false
	}
	switch strings.ToLower(s) {
	case "true", "false", "always", "ifnotpresent", "never", "null", "~":
		return false
	}
	return true
}
