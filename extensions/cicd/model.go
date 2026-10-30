package cicd

import "github.com/archstats/archstats/extensions/cicd/buildtool"

// Readers turn one file into facts; the linker joins facts across files into
// deployables. Paths inside facts are clean and repository-relative ("a/b",
// "." for the root); the walker's own names ("./x" at the root) are only
// restored when rows are written, so the snapshot's file columns match
// files.name.

// An imageBuild is a declaration that an image is built, as the build tools
// declare theirs.
type imageBuild = buildtool.Image

// An imageRef is something that runs an image.
type imageRef struct {
	Raw      string
	Name     ImageName
	Workload string // workload key
	File     string
	Line     int
	Source   string // compose, k8s, helm_values, kustomize, terraform, workflow
}

// A workload is something that runs: a compose service, a Kubernetes
// workload, a Helm release described by a values file.
type workload struct {
	Key       string // unique: file#name
	Name      string
	Kind      string
	File      string
	Line      int
	Namespace string
	Labels    map[string]string
	Images    []imageRef
	Build     *imageBuild
	Env       []configEntry
	EnvFrom   []string // ConfigMap names
	EnvFiles  []string // compose env_file paths, clean
	DependsOn []keyValue
	// Other names it is reachable by: container_name, hostname.
	Aliases []string
}

// A configEntry is one key and value of runtime configuration, wherever it
// was written.
type configEntry struct {
	Key   string
	Value string
	File  string
	Line  int
}

type k8sService struct {
	Name      string
	Namespace string
	Selector  map[string]string
	File      string
	Line      int
}

type configMap struct {
	Name string
	Data []configEntry
}

type kustomization struct {
	Dir       string
	File      string
	Resources []string // clean paths: files or directories
	Images    []kustomizeImage
	ConfigMap map[string][]configEntry
}

type kustomizeImage struct {
	Name, NewName, NewTag string
	Line                  int
}

type chart struct {
	Dir          string
	File         string
	Name         string
	Dependencies []chartDependency
}

type chartDependency struct {
	Name       string
	Repository string
	Line       int
	// A file:// dependency resolved to a directory in the workspace.
	LocalDir string
}

type valuesFile struct {
	File   string
	Dir    string
	Images []imageRef
	Flat   []flatValue
}

// A function is a serverless function whose code lives in the workspace.
type function struct {
	Name    string
	CodeDir string
	Handler string
	Runtime string
	File    string
	Line    int
	BuiltBy string // sam, serverless
}

// An argoApp is an Argo CD Application or one element of an ApplicationSet.
type argoApp struct {
	Name     string
	Path     string // clean path in this workspace, when the source is local
	RepoURL  string
	File     string
	Line     int
	Patterns []string // generators that make an open-ended number of apps
	Elements []string // enumerated environment-like names from a list generator
}
