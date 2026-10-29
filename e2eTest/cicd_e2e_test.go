package e2eTest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Artifacts and pipelines, checked against public systems chosen because
// each exercises one step that, skipped, silently yields nothing: Skaffold naming the images
// (microservices-demo), a Maven plugin inherited from a parent pom
// (spring-petclinic-microservices), compose interpolated from .env (the OTel
// demo), Jib modules and a shared ConfigMap (bank-of-anthos), an Aspire app
// host written in C# (eShop), and SAM functions (a serverless sample).
//
// Every expectation was measured on the pinned commit before the code that
// meets it was written; see archstats-ui tasks/cicd-plan.md.

type artifactRow struct {
	ID      string `csv:"ID"`
	Kind    string `csv:"KIND"`
	BuiltBy string `csv:"BUILT_BY"`
	Runtime string `csv:"RUNTIME"`
	Files   int    `csv:"FILES"`
}

func realArtifacts(t *testing.T, url, commit string) map[string]artifactRow {
	t.Helper()
	var rows []artifactRow
	realView(t, url, commit, "artifacts", "id,kind,built_by,runtime,files", &rows)
	out := map[string]artifactRow{}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

type pipelineJobRow struct {
	Pipeline string `csv:"PIPELINE"`
	ID       string `csv:"ID"`
	Needs    string `csv:"NEEDS"`
}

func realJobs(t *testing.T, url, commit string) map[string][]pipelineJobRow {
	t.Helper()
	var rows []pipelineJobRow
	realView(t, url, commit, "pipeline_jobs", "pipeline,id,needs", &rows)
	out := map[string][]pipelineJobRow{}
	for _, r := range rows {
		out[r.Pipeline] = append(out[r.Pipeline], r)
	}
	return out
}

func Test_Real_MicroservicesDemo_Artifacts(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/GoogleCloudPlatform/microservices-demo", "38e7348eb289eb5b87c0c6e8cb19ced0449dc389"
	ds := realArtifacts(t, url, commit)
	for _, name := range []string{"adservice", "cartservice", "checkoutservice", "currencyservice", "emailservice", "frontend", "loadgenerator", "paymentservice", "productcatalogservice", "recommendationservice", "shippingservice"} {
		d, ok := ds[name]
		if assert.Truef(t, ok, "%s is an artifact", name) {
			assert.Equal(t, "skaffold", d.BuiltBy, name)
			assert.NotEmpty(t, d.Runtime, name)
			assert.Positive(t, d.Files, name)
		}
	}
	jobs := realJobs(t, url, commit)
	assert.NotEmpty(t, jobs, "its GitHub workflows are read into jobs")
}

func Test_Real_SpringPetclinicMicroservices_Artifacts(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/spring-petclinic/spring-petclinic-microservices", "aefaf7fa9eb0ab911a24c346ca0e4dc5837251c7"
	ds := realArtifacts(t, url, commit)
	for _, svc := range []string{"admin-server", "api-gateway", "config-server", "customers-service", "discovery-server", "genai-service", "vets-service", "visits-service"} {
		d, ok := ds["spring-petclinic-"+svc]
		if assert.Truef(t, ok, "%s is built by the parent pom's docker profile", svc) {
			assert.Equal(t, "maven-docker", d.BuiltBy)
			assert.Equal(t, "java 17", d.Runtime)
		}
	}
}

func Test_Real_OpenTelemetryDemo_Artifacts(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/open-telemetry/opentelemetry-demo", "6a89faba15c8097c4bd82bcc15d906ac70baccd9"
	ds := realArtifacts(t, url, commit)
	assert.GreaterOrEqual(t, len(ds), 20, "images are named in .env and only join once compose is interpolated")
}

func Test_Real_BankOfAnthos_Artifacts(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/GoogleCloudPlatform/bank-of-anthos", "db35fea9fd090150e2398106aadb475576f80d94"
	ds := realArtifacts(t, url, commit)
	for _, jib := range []string{"balancereader", "ledgerwriter", "transactionhistory"} {
		d, ok := ds[jib]
		if assert.Truef(t, ok, "%s: one artifact, though Skaffold and the pom both declare it", jib) {
			assert.Equal(t, "jib", d.BuiltBy)
		}
	}
}

func Test_Real_EShop_Artifacts(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/dotnet/eShop", "b4a40872005d4bb29e5b1fa1ff7e244143d39215"
	ds := realArtifacts(t, url, commit)
	for _, app := range []string{"basket-api", "catalog-api", "identity-api", "order-processor", "ordering-api", "payment-processor", "webapp", "webhooks-api", "webhooksclient"} {
		d, ok := ds[app]
		if assert.Truef(t, ok, "%s is added by the Aspire app host", app) {
			assert.Equal(t, "aspire", d.BuiltBy)
		}
	}
}

func Test_Real_ServerlessShoppingCart_Artifacts(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/aws-samples/aws-serverless-shopping-cart", "66a863f1b7a2a7f319adddce6a55e090ce9f6734"
	ds := realArtifacts(t, url, commit)
	functions := 0
	for _, d := range ds {
		if d.Kind == "function" {
			functions++
			assert.Equal(t, "sam", d.BuiltBy)
		}
	}
	assert.Equal(t, 10, functions)
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
