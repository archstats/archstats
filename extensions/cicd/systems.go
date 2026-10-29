package cicd

import (
	stdpath "path"
	"strings"
)

// readOtherCI records a pipeline whose format is recognised but not read.
func readOtherCI(file string, system string) *pipelineFact {
	p := newPipeline(file, system)
	p.Read = "none"
	p.unread(1, 1, "not_supported_system")
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
	case base == ".rancher-pipeline.yml" || base == ".rancher-pipeline.yaml":
		return "rancher_pipelines"
	case base == ".drone.yml":
		return "drone"
	case strings.Contains(lower, ".tekton/"):
		return "tekton"
	case strings.Contains(lower, ".buildkite/"):
		return "buildkite"
	case strings.Contains(lower, ".woodpecker"):
		return "woodpecker"
	case strings.HasPrefix(base, "cloudbuild"):
		return "cloud_build"
	case strings.Contains(lower, ".github/actions/") || base == "action.yml" || base == "action.yaml":
		return "github_action"
	}
	return "other"
}
