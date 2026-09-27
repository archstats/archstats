package e2eTest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Deployables, checked against public systems chosen because each exercises
// one step that, skipped, silently yields nothing: Skaffold naming the images
// (microservices-demo), a Maven plugin inherited from a parent pom
// (spring-petclinic-microservices), compose interpolated from .env (the OTel
// demo), Jib modules and a shared ConfigMap (bank-of-anthos), an Aspire app
// host written in C# (eShop), and SAM functions (a serverless sample).
//
// Every expectation was measured on the pinned commit before the code that
// meets it was written; see archstats-ui tasks/deployables-plan.md.

type deployableRow struct {
	ID      string `csv:"ID"`
	Kind    string `csv:"KIND"`
	BuiltBy string `csv:"BUILT_BY"`
	Runtime string `csv:"RUNTIME"`
	Files   int    `csv:"FILES"`
}

type deployableLinkRow struct {
	From   string `csv:"FROM"`
	To     string `csv:"TO"`
	ToKind string `csv:"TO_KIND"`
	Kind   string `csv:"KIND"`
}

func realDeployables(t *testing.T, url, commit string) map[string]deployableRow {
	t.Helper()
	var rows []deployableRow
	realView(t, url, commit, "deployables", "id,kind,built_by,runtime,files", &rows)
	out := map[string]deployableRow{}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

func realDeployableLinks(t *testing.T, url, commit string) map[string]bool {
	t.Helper()
	var rows []deployableLinkRow
	realView(t, url, commit, "deployable_links", "from,to,to_kind,kind", &rows)
	out := map[string]bool{}
	for _, r := range rows {
		out[fmt.Sprintf("%s -%s-> %s", r.From, r.Kind, r.To)] = true
	}
	return out
}

func Test_Real_MicroservicesDemo_Deployables(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/GoogleCloudPlatform/microservices-demo", "38e7348eb289eb5b87c0c6e8cb19ced0449dc389"
	ds := realDeployables(t, url, commit)
	for _, name := range []string{"adservice", "cartservice", "checkoutservice", "currencyservice", "emailservice", "frontend", "loadgenerator", "paymentservice", "productcatalogservice", "recommendationservice", "shippingservice"} {
		d, ok := ds[name]
		if assert.Truef(t, ok, "%s is a deployable", name) {
			assert.Equal(t, "skaffold", d.BuiltBy, name)
			assert.NotEmpty(t, d.Runtime, name)
			assert.Positive(t, d.Files, name)
		}
	}
	links := realDeployableLinks(t, url, commit)
	for _, callee := range []string{"cartservice", "currencyservice", "emailservice", "paymentservice", "productcatalogservice", "shippingservice"} {
		assert.Truef(t, links["checkoutservice -calls-> "+callee], "checkoutservice calls %s", callee)
	}
	for _, callee := range []string{"adservice", "cartservice", "checkoutservice", "currencyservice", "productcatalogservice", "recommendationservice", "shippingservice"} {
		assert.Truef(t, links["frontend -calls-> "+callee], "frontend calls %s", callee)
	}
	assert.True(t, links["cartservice -calls-> redis-cart"], "redis-cart runs an image not built here")
	calls := 0
	for l := range links {
		if len(l) > 0 && containsStr(l, "-calls->") {
			calls++
		}
	}
	assert.GreaterOrEqual(t, calls, 16)
}

func Test_Real_SpringPetclinicMicroservices_Deployables(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/spring-petclinic/spring-petclinic-microservices", "aefaf7fa9eb0ab911a24c346ca0e4dc5837251c7"
	ds := realDeployables(t, url, commit)
	for _, svc := range []string{"admin-server", "api-gateway", "config-server", "customers-service", "discovery-server", "genai-service", "vets-service", "visits-service"} {
		d, ok := ds["spring-petclinic-"+svc]
		if assert.Truef(t, ok, "%s is built by the parent pom's docker profile", svc) {
			assert.Equal(t, "maven-docker", d.BuiltBy)
			assert.Equal(t, "java 17", d.Runtime)
		}
	}
	links := realDeployableLinks(t, url, commit)
	for _, svc := range []string{"customers-service", "vets-service", "visits-service", "genai-service"} {
		assert.Truef(t, links["spring-petclinic-api-gateway -calls-> spring-petclinic-"+svc], "the gateway routes to %s", svc)
	}
	toConfig := 0
	for l := range links {
		if containsStr(l, "-calls-> spring-petclinic-config-server") {
			toConfig++
		}
	}
	assert.GreaterOrEqual(t, toConfig, 7, "every service reads the config server")
}

func Test_Real_OpenTelemetryDemo_Deployables(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/open-telemetry/opentelemetry-demo", "6a89faba15c8097c4bd82bcc15d906ac70baccd9"
	ds := realDeployables(t, url, commit)
	assert.GreaterOrEqual(t, len(ds), 20, "images are named in .env and only join once compose is interpolated")
	links := realDeployableLinks(t, url, commit)
	for _, callee := range []string{"cart", "currency", "email", "payment", "product-catalog", "shipping"} {
		assert.Truef(t, links["checkout -calls-> "+callee], "checkout calls %s (addresses come from .env)", callee)
	}
}

func Test_Real_BankOfAnthos_Deployables(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/GoogleCloudPlatform/bank-of-anthos", "db35fea9fd090150e2398106aadb475576f80d94"
	ds := realDeployables(t, url, commit)
	for _, jib := range []string{"balancereader", "ledgerwriter", "transactionhistory"} {
		d, ok := ds[jib]
		if assert.Truef(t, ok, "%s: one deployable, though Skaffold and the pom both declare it", jib) {
			assert.Equal(t, "jib", d.BuiltBy)
		}
	}
	links := realDeployableLinks(t, url, commit)
	assert.True(t, links["frontend -calls-> userservice"])
	assert.True(t, links["frontend -calls-> balancereader"])
	assert.True(t, links["contacts -uses_datastore-> accounts-db"], "a database image built here is a deployable")
	assert.True(t, links["contacts -shares_datastore-> userservice"])
}

func Test_Real_EShop_Deployables(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/dotnet/eShop", "b4a40872005d4bb29e5b1fa1ff7e244143d39215"
	ds := realDeployables(t, url, commit)
	for _, app := range []string{"basket-api", "catalog-api", "identity-api", "order-processor", "ordering-api", "payment-processor", "webapp", "webhooks-api", "webhooksclient"} {
		d, ok := ds[app]
		if assert.Truef(t, ok, "%s is added by the Aspire app host", app) {
			assert.Equal(t, "aspire", d.BuiltBy)
		}
	}
	links := realDeployableLinks(t, url, commit)
	for _, callee := range []string{"basket-api", "catalog-api", "ordering-api", "identity-api"} {
		assert.Truef(t, links["webapp -calls-> "+callee], "webapp references %s", callee)
	}
	assert.True(t, links["order-processor -shares_datastore-> ordering-api"], "both reference orderingdb")
}

func Test_Real_ServerlessShoppingCart_Deployables(t *testing.T) {
	skipIfShort(t)
	const url, commit = "https://github.com/aws-samples/aws-serverless-shopping-cart", "66a863f1b7a2a7f319adddce6a55e090ce9f6734"
	ds := realDeployables(t, url, commit)
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
