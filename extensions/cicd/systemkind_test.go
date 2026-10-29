package cicd

import (
	"testing"

	"github.com/archstats/archstats/core/file"
)

// Paths taken from the repositories measured while designing this: LibreChat,
// Sylius, Fineract, microservices-demo, bank-of-anthos, the OTel demo,
// spring-petclinic-microservices, eShop, a SAM sample and an enterprise
// workspace whose deployment lives only in per-environment values files.
func TestSystemKindByPath(t *testing.T) {
	cases := map[string]string{
		// lockfiles win over everything
		"./package-lock.json":        file.SystemKindLockfile,
		"client/yarn.lock":           file.SystemKindLockfile,
		"pnpm-lock.yaml":             file.SystemKindLockfile,
		"./go.sum":                   file.SystemKindLockfile,
		"packages/api/poetry.lock":   file.SystemKindLockfile,
		"src/Web/packages.lock.json": file.SystemKindLockfile,
		"composer.lock":              file.SystemKindLockfile,
		"Cargo.lock":                 file.SystemKindLockfile,
		"app/gradle.lockfile":        file.SystemKindLockfile,
		"Gemfile.lock":               file.SystemKindLockfile,
		"uv.lock":                    file.SystemKindLockfile,

		// pipelines
		".github/workflows/main-image-workflow.yml":      file.SystemKindCI,
		".github/workflows/bunnyshell_cleanup-envs.yaml": file.SystemKindCI,
		".github/actions/setup/action.yml":               file.SystemKindCI,
		"./Jenkinsfile":                                  file.SystemKindCI,
		"ci/Jenkinsfile.release":                         file.SystemKindCI,
		"nightly.jenkinsfile":                            file.SystemKindCI,
		"Jenkinsfile-nightly":                            file.SystemKindCI,
		"./.gitlab-ci.yml":                               file.SystemKindCI,
		".gitlab/ci/test.gitlab-ci.yml":                  file.SystemKindCI,
		".travis.yml":                                    file.SystemKindCI,
		".circleci/config.yml":                           file.SystemKindCI,
		"azure-pipelines.yml":                            file.SystemKindCI,
		"build/azure-pipelines-release.yaml":             file.SystemKindCI,
		"bitbucket-pipelines.yml":                        file.SystemKindCI,
		"cloudbuild.yaml":                                file.SystemKindCI,
		".buildkite/pipeline.yml":                        file.SystemKindCI,
		".drone.yml":                                     file.SystemKindCI,
		"buildspec.yml":                                  file.SystemKindCI,

		// containers
		"./Dockerfile":                   file.SystemKindContainer,
		"Dockerfile.multi":               file.SystemKindContainer,
		"src/cartservice/src/Dockerfile": file.SystemKindContainer,
		".devcontainer/Dockerfile":       file.SystemKindContainer,
		"docker/dev.Dockerfile":          file.SystemKindContainer,
		"Dockerfile-alpine":              file.SystemKindContainer,
		"build/Containerfile":            file.SystemKindContainer,
		"images/base.containerfile":      file.SystemKindContainer,

		// deployment
		"docker-compose.yml":                            file.SystemKindDeploy,
		"./compose.yaml":                                file.SystemKindDeploy,
		"compose.full.yaml":                             file.SystemKindDeploy,
		"docker-compose.override.yml":                   file.SystemKindDeploy,
		"deploy-compose.yml":                            "", // named like nothing; content decides
		"helm/librechat/Chart.yaml":                     file.SystemKindDeploy,
		"helm/librechat/values.yaml":                    file.SystemKindDeploy,
		"charts/api/values-prod.yaml":                   file.SystemKindDeploy,
		"values.dev.yml":                                file.SystemKindDeploy,
		"prod.values.yaml":                              file.SystemKindDeploy,
		"helm-release/nonprod-dev.yaml":                 file.SystemKindDeploy,
		"helm-release/prod.yaml":                        file.SystemKindDeploy,
		"kustomize/overlays/staging/kustomization.yaml": file.SystemKindDeploy,
		"kubernetes-manifests/cartservice.yaml":         file.SystemKindDeploy, // by folder
		"k8s/base/deployment.yml":                       file.SystemKindDeploy,
		"deploy/gcp/service.yaml":                       file.SystemKindDeploy,
		"serverless.yml":                                file.SystemKindDeploy,
		"Procfile":                                      file.SystemKindDeploy,
		"fly.toml":                                      file.SystemKindDeploy,
		"cdk.json":                                      file.SystemKindDeploy,
		"Pulumi.yaml":                                   file.SystemKindDeploy,
		"Tiltfile":                                      file.SystemKindDeploy,

		// build
		"./pom.xml":                              file.SystemKindBuild,
		"fineract-provider/build.gradle":         file.SystemKindBuild,
		"settings.gradle.kts":                    file.SystemKindBuild,
		"gradle/libs.versions.toml":              file.SystemKindBuild,
		"packages/data-provider/package.json":    file.SystemKindBuild,
		"./go.mod":                               file.SystemKindBuild,
		"src/Libraries/Nop.Core/Nop.Core.csproj": file.SystemKindBuild,
		"eShop.sln":                              file.SystemKindBuild,
		"Directory.Packages.props":               file.SystemKindBuild,
		"composer.json":                          file.SystemKindBuild,
		"pyproject.toml":                         file.SystemKindBuild,
		"requirements.txt":                       file.SystemKindBuild,
		"requirements-dev.txt":                   file.SystemKindBuild,
		"requirements/base.txt":                  file.SystemKindBuild,
		"Makefile":                               file.SystemKindBuild,
		"BUILD.bazel":                            file.SystemKindBuild,
		"MODULE.bazel":                           file.SystemKindBuild,
		".goreleaser.yml":                        file.SystemKindBuild,
		"nx.json":                                file.SystemKindBuild,
		"turbo.json":                             file.SystemKindBuild,
		"Cargo.toml":                             file.SystemKindBuild,

		// infrastructure
		"terraform/upload_job.tf":    file.SystemKindInfra,
		"envs/prod/terraform.tfvars": file.SystemKindInfra,
		"live/prod/terragrunt.hcl":   file.SystemKindInfra,
		"infra/main.bicep":           file.SystemKindInfra,

		// runtime configuration
		"src/main/resources/application.properties": file.SystemKindConfig,
		"src/main/resources/application-prod.yml":   file.SystemKindConfig,
		"src/main/resources/bootstrap.properties":   file.SystemKindConfig,
		"src/Web/appsettings.Development.json":      file.SystemKindConfig,
		"./.env":                                    file.SystemKindConfig,
		".env.example":                              file.SystemKindConfig,
		"config/prod.env":                           file.SystemKindConfig,

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
		{"k8s deployment anywhere", "ops/cartservice.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: cart\n", file.SystemKindDeploy},
		{"k8s multi-document", "misc/all.yml", "# comment\n---\napiVersion: v1\nkind: Service\n---\napiVersion: apps/v1\nkind: Deployment\n", file.SystemKindDeploy},
		{"argo application", "apps/guestbook.yaml", "apiVersion: argoproj.io/v1alpha1\nkind: Application\n", file.SystemKindDeploy},
		{"helm template", "chart/templates/deployment.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: {{ .Release.Name }}\n", file.SystemKindDeploy},
		{"skaffold", "skaffold-e2e.yaml", "apiVersion: skaffold/v4beta1\nkind: Config\n", file.SystemKindDeploy},
		{"compose under another name", "deploy-compose.yml", "version: '3'\nservices:\n  api:\n    image: ghcr.io/x/api:latest\n", file.SystemKindDeploy},
		{"compose with build first", "stack.yml", "services:\n  web:\n    container_name: web\n    build: .\n", file.SystemKindDeploy},
		{"SAM template", "template.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nTransform: AWS::Serverless-2016-10-31\nResources: {}\n", file.SystemKindDeploy},
		{"CloudFormation yaml", "stacks/network.yaml", "AWSTemplateFormatVersion: \"2010-09-09\"\nResources:\n  Vpc:\n    Type: AWS::EC2::VPC\n", file.SystemKindInfra},
		{"CloudFormation json", "cdk.out/App.template.json", "{\n  \"Resources\": {\n    \"Q\": {\n      \"Type\": \"AWS::SQS::Queue\"\n    }\n  }\n}", file.SystemKindInfra},
		{"workflow outside .github", "templates/ci.yml", "name: CI\non:\n  push:\njobs:\n  build:\n    runs-on: ubuntu-latest\n", file.SystemKindCI},
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
	if got := SystemKind(".github/workflows/deploy.yml", []byte("apiVersion: apps/v1\nkind: Deployment\n")); got != file.SystemKindCI {
		t.Errorf("workflow: %q", got)
	}
	if got := SystemKind("charts/x/values.yaml", nil); got != file.SystemKindDeploy {
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
