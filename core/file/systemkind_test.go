package file

import "testing"

// Paths taken from the repositories measured while designing this: LibreChat,
// Sylius, Fineract, microservices-demo, bank-of-anthos, the OTel demo,
// spring-petclinic-microservices, eShop, a SAM sample and an enterprise
// workspace whose deployment lives only in per-environment values files.
func TestSystemKindByPath(t *testing.T) {
	cases := map[string]string{
		// lockfiles win over everything
		"./package-lock.json":        SystemKindLockfile,
		"client/yarn.lock":           SystemKindLockfile,
		"pnpm-lock.yaml":             SystemKindLockfile,
		"./go.sum":                   SystemKindLockfile,
		"packages/api/poetry.lock":   SystemKindLockfile,
		"src/Web/packages.lock.json": SystemKindLockfile,
		"composer.lock":              SystemKindLockfile,
		"Cargo.lock":                 SystemKindLockfile,
		"app/gradle.lockfile":        SystemKindLockfile,
		"Gemfile.lock":               SystemKindLockfile,
		"uv.lock":                    SystemKindLockfile,

		// pipelines
		".github/workflows/main-image-workflow.yml":      SystemKindCI,
		".github/workflows/bunnyshell_cleanup-envs.yaml": SystemKindCI,
		".github/actions/setup/action.yml":               SystemKindCI,
		"./Jenkinsfile":                                  SystemKindCI,
		"ci/Jenkinsfile.release":                         SystemKindCI,
		"nightly.jenkinsfile":                            SystemKindCI,
		"Jenkinsfile-nightly":                            SystemKindCI,
		"./.gitlab-ci.yml":                               SystemKindCI,
		".gitlab/ci/test.gitlab-ci.yml":                  SystemKindCI,
		".travis.yml":                                    SystemKindCI,
		".circleci/config.yml":                           SystemKindCI,
		"azure-pipelines.yml":                            SystemKindCI,
		"build/azure-pipelines-release.yaml":             SystemKindCI,
		"bitbucket-pipelines.yml":                        SystemKindCI,
		"cloudbuild.yaml":                                SystemKindCI,
		".buildkite/pipeline.yml":                        SystemKindCI,
		".drone.yml":                                     SystemKindCI,
		"buildspec.yml":                                  SystemKindCI,

		// containers
		"./Dockerfile":                   SystemKindContainer,
		"Dockerfile.multi":               SystemKindContainer,
		"src/cartservice/src/Dockerfile": SystemKindContainer,
		".devcontainer/Dockerfile":       SystemKindContainer,
		"docker/dev.Dockerfile":          SystemKindContainer,
		"Dockerfile-alpine":              SystemKindContainer,
		"build/Containerfile":            SystemKindContainer,
		"images/base.containerfile":      SystemKindContainer,

		// deployment
		"docker-compose.yml":                            SystemKindDeploy,
		"./compose.yaml":                                SystemKindDeploy,
		"compose.full.yaml":                             SystemKindDeploy,
		"docker-compose.override.yml":                   SystemKindDeploy,
		"deploy-compose.yml":                            "", // named like nothing; content decides
		"helm/librechat/Chart.yaml":                     SystemKindDeploy,
		"helm/librechat/values.yaml":                    SystemKindDeploy,
		"charts/api/values-prod.yaml":                   SystemKindDeploy,
		"values.dev.yml":                                SystemKindDeploy,
		"prod.values.yaml":                              SystemKindDeploy,
		"helm-release/nonprod-dev.yaml":                 SystemKindDeploy,
		"helm-release/prod.yaml":                        SystemKindDeploy,
		"kustomize/overlays/staging/kustomization.yaml": SystemKindDeploy,
		"kubernetes-manifests/cartservice.yaml":         SystemKindDeploy, // by folder
		"k8s/base/deployment.yml":                       SystemKindDeploy,
		"deploy/gcp/service.yaml":                       SystemKindDeploy,
		"serverless.yml":                                SystemKindDeploy,
		"Procfile":                                      SystemKindDeploy,
		"fly.toml":                                      SystemKindDeploy,
		"cdk.json":                                      SystemKindDeploy,
		"Pulumi.yaml":                                   SystemKindDeploy,
		"Tiltfile":                                      SystemKindDeploy,

		// build
		"./pom.xml":                              SystemKindBuild,
		"fineract-provider/build.gradle":         SystemKindBuild,
		"settings.gradle.kts":                    SystemKindBuild,
		"gradle/libs.versions.toml":              SystemKindBuild,
		"packages/data-provider/package.json":    SystemKindBuild,
		"./go.mod":                               SystemKindBuild,
		"src/Libraries/Nop.Core/Nop.Core.csproj": SystemKindBuild,
		"eShop.sln":                              SystemKindBuild,
		"Directory.Packages.props":               SystemKindBuild,
		"composer.json":                          SystemKindBuild,
		"pyproject.toml":                         SystemKindBuild,
		"requirements.txt":                       SystemKindBuild,
		"requirements-dev.txt":                   SystemKindBuild,
		"requirements/base.txt":                  SystemKindBuild,
		"Makefile":                               SystemKindBuild,
		"BUILD.bazel":                            SystemKindBuild,
		"MODULE.bazel":                           SystemKindBuild,
		".goreleaser.yml":                        SystemKindBuild,
		"nx.json":                                SystemKindBuild,
		"turbo.json":                             SystemKindBuild,
		"Cargo.toml":                             SystemKindBuild,

		// infrastructure
		"terraform/upload_job.tf":    SystemKindInfra,
		"envs/prod/terraform.tfvars": SystemKindInfra,
		"live/prod/terragrunt.hcl":   SystemKindInfra,
		"infra/main.bicep":           SystemKindInfra,

		// runtime configuration
		"src/main/resources/application.properties": SystemKindConfig,
		"src/main/resources/application-prod.yml":   SystemKindConfig,
		"src/main/resources/bootstrap.properties":   SystemKindConfig,
		"src/Web/appsettings.Development.json":      SystemKindConfig,
		"./.env":                                    SystemKindConfig,
		".env.example":                              SystemKindConfig,
		"config/prod.env":                           SystemKindConfig,

		// not system files
		"src/main/java/com/acme/Order.java":      "",
		"client/src/App.tsx":                     "",
		"README.md":                              "",
		"docs/deploy.md":                         "",
		"src/main/resources/messages.properties": "",
		"tsconfig.json":                          "",
		"src/i18n/en.json":                       "",
		"config/locales/fr.yml":                  "",
		".github/dependabot.yml":                 "",
		".github/CODEOWNERS":                     "",
		"venv":                                   "",
		"src/build/Builder.java":                 "",
		"docs/images/docker.png":                 "",
		"application.java":                       "",
	}
	for p, want := range cases {
		if got := SystemKind(p, nil); got != want {
			t.Errorf("%s: %q, want %q", p, got, want)
		}
	}
}

func TestSystemKindByContent(t *testing.T) {
	cases := []struct {
		name, path, content, want string
	}{
		{"k8s deployment anywhere", "ops/cartservice.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: cart\n", SystemKindDeploy},
		{"k8s multi-document", "misc/all.yml", "# comment\n---\napiVersion: v1\nkind: Service\n---\napiVersion: apps/v1\nkind: Deployment\n", SystemKindDeploy},
		{"argo application", "apps/guestbook.yaml", "apiVersion: argoproj.io/v1alpha1\nkind: Application\n", SystemKindDeploy},
		{"helm template", "chart/templates/deployment.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: {{ .Release.Name }}\n", SystemKindDeploy},
		{"skaffold", "skaffold-e2e.yaml", "apiVersion: skaffold/v4beta1\nkind: Config\n", SystemKindDeploy},
		{"compose under another name", "deploy-compose.yml", "version: '3'\nservices:\n  api:\n    image: ghcr.io/x/api:latest\n", SystemKindDeploy},
		{"compose with build first", "stack.yml", "services:\n  web:\n    container_name: web\n    build: .\n", SystemKindDeploy},
		{"SAM template", "template.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nTransform: AWS::Serverless-2016-10-31\nResources: {}\n", SystemKindDeploy},
		{"CloudFormation yaml", "stacks/network.yaml", "AWSTemplateFormatVersion: \"2010-09-09\"\nResources:\n  Vpc:\n    Type: AWS::EC2::VPC\n", SystemKindInfra},
		{"CloudFormation json", "cdk.out/App.template.json", "{\n  \"Resources\": {\n    \"Q\": {\n      \"Type\": \"AWS::SQS::Queue\"\n    }\n  }\n}", SystemKindInfra},
		{"workflow outside .github", "templates/ci.yml", "name: CI\non:\n  push:\njobs:\n  build:\n    runs-on: ubuntu-latest\n", SystemKindCI},
		{"openapi is not deployment", "api/openapi.yaml", "openapi: 3.0.0\ninfo:\n  title: x\n", ""},
		{"plain yaml", "config/features.yml", "flags:\n  beta: true\n", ""},
		{"services key that is not compose", "config/services.yaml", "services:\n  mailer:\n    class: App\\Mailer\n", ""},
		{"i18n json", "i18n/en.json", "{\"hello\": \"Hello\"}", ""},
	}
	for _, c := range cases {
		if got := SystemKind(c.path, []byte(c.content)); got != c.want {
			t.Errorf("%s (%s): %q, want %q", c.name, c.path, got, c.want)
		}
	}
}

// A path rule beats content: a workflow is ci even though it holds no
// apiVersion, and a values file in a chart is deploy even when it is empty.
func TestSystemKindPathBeatsContent(t *testing.T) {
	if got := SystemKind(".github/workflows/deploy.yml", []byte("apiVersion: apps/v1\nkind: Deployment\n")); got != SystemKindCI {
		t.Errorf("workflow: %q", got)
	}
	if got := SystemKind("charts/x/values.yaml", nil); got != SystemKindDeploy {
		t.Errorf("empty values: %q", got)
	}
}

// Only the head of a large file is read.
func TestSystemKindReadsOnlyTheHead(t *testing.T) {
	big := make([]byte, 20000)
	for i := range big {
		big[i] = ' '
	}
	content := append(big, []byte("\napiVersion: apps/v1\nkind: Deployment\n")...)
	if got := SystemKind("misc/late.yaml", content); got != "" {
		t.Errorf("a marker beyond the head must not count: %q", got)
	}
}
