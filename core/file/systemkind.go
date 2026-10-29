package file

// A file's system kind says what part of building, testing, shipping or
// running the software it describes, when it describes any. It is a second
// axis next to Role: a GitHub workflow is non_code by role and ci by kind; a
// Dockerfile is production by role and container by kind. Most files have no
// system kind at all.
//
// Core holds the vocabulary. Which file is which kind is registered as
// Conventions: CI/CD names pipelines, containers, deployment and build files,
// mobile names app manifests.
const (
	SystemKindBuild     = "build"     // what the build tool builds: pom.xml, package.json, go.mod
	SystemKindLockfile  = "lockfile"  // what the build resolved: package-lock.json, go.sum
	SystemKindCI        = "ci"        // pipelines: workflows, Jenkinsfile, .gitlab-ci.yml
	SystemKindContainer = "container" // Dockerfile, Containerfile
	SystemKindDeploy    = "deploy"    // compose, Kubernetes, Helm, kustomize, Skaffold, SAM
	SystemKindInfra     = "infra"     // Terraform, CloudFormation, Terragrunt
	SystemKindConfig    = "config"    // runtime configuration: application.yml, .env, appsettings.json
	// What an app declares to the platform it installs on: AndroidManifest.xml,
	// Info.plist, entitlements. Its activities, permissions and URL schemes
	// are the mobile equivalent of a service's routes.
	SystemKindAppManifest = "app_manifest"
)
