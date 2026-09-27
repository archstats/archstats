package deployables

import (
	"reflect"
	"strings"
	"testing"
)

func TestReadProperties(t *testing.T) {
	src := "# comment\n! also a comment\n\nspring.application.name=booking\nserver.port: 8080\nplain value here\nmulti.line=first \\\n    second\nempty=\n  indented.key = spaced value \nkey.only\n"
	got := readProperties("application.properties", []byte(src))
	want := []configEntry{
		{Key: "spring.application.name", Value: "booking", File: "application.properties", Line: 4},
		{Key: "server.port", Value: "8080", File: "application.properties", Line: 5},
		{Key: "plain", Value: "value here", File: "application.properties", Line: 6},
		{Key: "multi.line", Value: "first second", File: "application.properties", Line: 7},
		{Key: "empty", Value: "", File: "application.properties", Line: 9},
		{Key: "indented.key", Value: "spaced value", File: "application.properties", Line: 10},
		{Key: "key.only", Value: "", File: "application.properties", Line: 11},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestReadConfigYAMLProfiles(t *testing.T) {
	src := `spring:
  application:
    name: customers-service
  config:
    import: optional:configserver:${CONFIG_SERVER_URL:http://localhost:8888/}
---
spring:
  config:
    activate:
      on-profile: docker
    import: configserver:http://config-server:8888
`
	entries, profiles := readConfigYAML("application.yml", []byte(src))
	if applicationName(entries) != "customers-service" {
		t.Errorf("name = %q", applicationName(entries))
	}
	var dockerLine int
	for _, e := range entries {
		if e.Key == "spring.config.import" && strings.Contains(e.Value, "config-server") {
			dockerLine = e.Line
		}
	}
	if profiles[dockerLine] != "docker" {
		t.Errorf("profile at line %d = %q", dockerLine, profiles[dockerLine])
	}
}

func TestSpringProfileOf(t *testing.T) {
	cases := map[string]string{
		"src/main/resources/application.yml":            "",
		"src/main/resources/application-prod.yml":       "prod",
		"src/main/resources/application-dev.properties": "dev",
		"bootstrap-staging.yml":                         "staging",
		"appsettings.Development.json":                  "Development",
		"appsettings.json":                              "",
	}
	for in, want := range cases {
		if got := springProfileOf(in); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

func TestIsSecretKey(t *testing.T) {
	secret := []string{"spring.datasource.password", "DB_PASSWORD", "api.express.connect.pricing.clientSecret", "API_KEY", "apiKey", "jwt.token", "aws.accessKey", "passwordAus", "ConnectionStrings:Default", "SIGNING_KEY", "private-key"}
	plain := []string{"spring.datasource.url", "server.port", "topic.name", "feature.flags", "keycloak.realm", "monkey.count"}
	for _, k := range secret {
		if !isSecretKey(k) {
			t.Errorf("%s should be secret", k)
		}
	}
	for _, k := range plain {
		if isSecretKey(k) {
			t.Errorf("%s should not be secret", k)
		}
	}
}

func TestClassify(t *testing.T) {
	type c struct {
		key, value string
		want       []endpoint
	}
	cases := []c{
		{"PRODUCT_CATALOG_SERVICE_ADDR", "productcatalogservice:3550", []endpoint{{Kind: "http", Host: "productcatalogservice", Port: "3550", Internal: true}}},
		{"CART_ADDR", "cart.default.svc.cluster.local:7070", []endpoint{{Kind: "http", Host: "cart", Port: "7070", Internal: true}}},
		{"spring.cloud.gateway.routes[0].uri", "lb://customers-service", []endpoint{{Kind: "http", Host: "customers-service", Internal: true}}},
		{"spring.config.import", "optional:configserver:${CONFIG_SERVER_URL:http://config-server:8888/}", []endpoint{{Kind: "http", Host: "config-server", Port: "8888", Internal: true}}},
		{"spring.config.import", "optional:configserver:${CONFIG_SERVER_URL:http://localhost:8888/}", nil},
		{"api.pricing.url", "https://api.partner.example.io/v2/rates", []endpoint{{Kind: "http", Host: "api.partner.example.io"}}},
		{"spring.datasource.url", "jdbc:postgresql://db-host.acme.internal:5432/quotes?ssl=true", []endpoint{{Kind: "datastore", Vendor: "postgresql", Host: "db-host.acme.internal", Port: "5432", Database: "quotes"}}},
		{"spring.datasource.url", "jdbc:postgresql://postgres/catalog", []endpoint{{Kind: "datastore", Vendor: "postgresql", Host: "postgres", Database: "catalog", Internal: true}}},
		{"DB", "jdbc:sqlserver://sql01:1433;databaseName=Orders;encrypt=true", []endpoint{{Kind: "datastore", Vendor: "sqlserver", Host: "sql01", Port: "1433", Database: "Orders", Internal: true}}},
		{"DB", "jdbc:oracle:thin:@ora-host:1521:ORCL", []endpoint{{Kind: "datastore", Vendor: "oracle", Host: "ora-host", Port: "1521", Database: "ORCL", Internal: true}}},
		{"DB", "jdbc:oracle:thin:@//ora-host:1521/svc", []endpoint{{Kind: "datastore", Vendor: "oracle", Host: "ora-host", Port: "1521", Database: "svc", Internal: true}}},
		{"MONGO_URI", "mongodb://user:secret@mongo:27017/librechat?authSource=admin", []endpoint{{Kind: "datastore", Vendor: "mongodb", Host: "mongo", Port: "27017", Database: "librechat", Internal: true}}},
		{"MONGO_URI", "mongodb://a:27017,b:27017/app", []endpoint{{Kind: "datastore", Vendor: "mongodb", Host: "a", Port: "27017", Database: "app", Internal: true}}},
		{"REDIS_ADDR", "redis://redis-cart:6379/0", []endpoint{{Kind: "datastore", Vendor: "redis", Host: "redis-cart", Port: "6379", Internal: true}}},
		{"spring.kafka.bootstrap-servers", "kafka1:9092,kafka2:9092", []endpoint{{Kind: "broker", Vendor: "kafka", Host: "kafka1", Port: "9092", Internal: true}, {Kind: "broker", Vendor: "kafka", Host: "kafka2", Port: "9092", Internal: true}}},
		{"RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/", []endpoint{{Kind: "broker", Vendor: "amqp", Host: "rabbitmq", Port: "5672", Internal: true}}},
		{"topic.name", "qp_topic_dev", []endpoint{{Kind: "topic", Topic: "qp_topic_dev"}}},
		{"spring.cloud.stream.bindings.orders-out-0.destination", "orders", []endpoint{{Kind: "topic", Topic: "orders"}}},
		{"KAFKA_TOPIC", "orders,payments", []endpoint{{Kind: "topic", Topic: "orders"}, {Kind: "topic", Topic: "payments"}}},
		{"ORDER_QUEUE_NAME", "order-events", []endpoint{{Kind: "topic", Topic: "order-events"}}},
		{"topic.group.id", "qp_group_id_dev", nil},
		{"kafka.topic.partitions", "3", nil},
		{"server.port", "8080", nil},
		{"feature.enabled", "true", nil},
		{"LISTEN_ADDR", "0.0.0.0:8080", nil},
		{"API", "http://127.0.0.1:3000", nil},
		{"schema", "http://www.w3.org/2001/XMLSchema", nil},
		{"TEMPLATE", "{{ .Values.host }}:8080", nil},
		{"UNRESOLVED", "${SOME_HOST}:8080", nil},
		{"time", "12:30", nil},
	}
	for _, tc := range cases {
		got := classify(tc.key, tc.value)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s=%s:\n got %+v\nwant %+v", tc.key, tc.value, got, tc.want)
		}
	}
}

func TestNormalizeHost(t *testing.T) {
	cases := map[string][2]string{
		"cartservice":                 {"cartservice", "internal"},
		"CartService":                 {"cartservice", "internal"},
		"cart.prod.svc.cluster.local": {"cart", "internal"},
		"cart.prod.svc":               {"cart", "internal"},
		"api.stripe.com":              {"api.stripe.com", "external"},
		"localhost":                   {"", ""},
		"10.0.0.4":                    {"", ""},
		"host.docker.internal":        {"", ""},
		"docs.example.com":            {"", ""},
	}
	for in, want := range cases {
		h, internal := normalizeHost(in)
		gotKind := ""
		if h != "" {
			gotKind = "external"
			if internal {
				gotKind = "internal"
			}
		}
		if h != want[0] || gotKind != want[1] {
			t.Errorf("%s: %q %s, want %v", in, h, gotKind, want)
		}
	}
}

func TestSecretReferencesAreNotSecrets(t *testing.T) {
	for _, k := range []string{"extraVars.15.valueFrom.secretKeyRef.name", "env.0.valueFrom.secretKeyRef.key", "auth.existingSecret", "db.secretName"} {
		if isSecretKey(k) {
			t.Errorf("%s names a secret kept elsewhere", k)
		}
	}
	if !isSecretKey("extraVars.3.value.clientSecret") {
		t.Error("a client secret is a secret")
	}
}

func TestBareHostPortNeedsAnAddressKey(t *testing.T) {
	cases := map[string]int{
		"pricing.rule=pickup:10":                 0,
		"CART_ADDR=cart:7070":                    1,
		"shipping.service=shipping:8080":         1,
		"backend.hosts=a:8080,b:8080":            2,
		"server.port=8080":                       0,
		"ratio=surcharge:25":                     0,
		"eureka.url=http://registry:8761/eureka": 1,
	}
	for kv, want := range cases {
		k, v, _ := strings.Cut(kv, "=")
		if got := len(classify(k, v)); got != want {
			t.Errorf("%s: %d endpoints, want %d", kv, got, want)
		}
	}
}

func TestStripEnvironment(t *testing.T) {
	cases := map[string]string{
		"qp-webform-test":        "qp-webform",
		"qp-webform-staging":     "qp-webform",
		"qp-webform-development": "qp-webform",
		"orders-prod2":           "orders",
		"dev-orders":             "orders",
		"orders":                 "orders",
		"contest-api":            "contest-api",
	}
	for in, want := range cases {
		if got := stripEnvironment(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}
