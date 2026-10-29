package cicd

import (
	"sort"
	"strings"

	"github.com/archstats/archstats/extensions/cicd/buildtool"
)

// Joining by name is the weakest evidence the linker has, and where it goes
// wrong. Go import resolution once tail-matched testify's `assert` onto
// archstats' own `cmd/assert`; an image called `mongo` must never land on a
// folder that happens to be called mongo. So: normalise hard, match exactly,
// and refuse a tie rather than pick a winner.

// ImageName and NormalizeImage are the contract's, shared with the build
// tools that name images too.
type ImageName = buildtool.ImageName

var NormalizeImage = buildtool.NormalizeImage

func isInterpolated(s string) bool { return buildtool.IsInterpolated(s) }

// normalizeName is how service, workload and application names are compared.
func normalizeName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// A nameIndex maps join keys to the things that answer to them.
type nameIndex struct {
	byKey map[string][]string
}

func newNameIndex() *nameIndex { return &nameIndex{byKey: map[string][]string{}} }

func (x *nameIndex) add(key, id string) {
	key = normalizeName(key)
	if key == "" {
		return
	}
	for _, have := range x.byKey[key] {
		if have == id {
			return
		}
	}
	x.byKey[key] = append(x.byKey[key], id)
	sort.Strings(x.byKey[key])
}

// A lookup either finds exactly one thing, finds several (a tie, refused),
// or finds nothing.
type lookup struct {
	ID        string
	Ambiguous []string
}

func (l lookup) found() bool { return l.ID != "" }

func (x *nameIndex) find(key string) lookup {
	ids := x.byKey[normalizeName(key)]
	switch len(ids) {
	case 0:
		return lookup{}
	case 1:
		return lookup{ID: ids[0]}
	}
	return lookup{Ambiguous: ids}
}

// findImage tries repo:tag before repo, so images told apart only by tag
// still join, and an exact tag never loses to a bare-name tie.
func (x *nameIndex) findImage(n ImageName) lookup {
	if n.Repo == "" {
		return lookup{}
	}
	if n.Tag != "" {
		if l := x.find(n.Repo + ":" + n.Tag); l.found() || len(l.Ambiguous) > 0 {
			return l
		}
	}
	return x.find(n.Repo)
}

func (x *nameIndex) addImage(n ImageName, id string) {
	if n.Repo == "" {
		return
	}
	x.add(n.Repo, id)
	if n.Tag != "" && n.Tag != "latest" {
		x.add(n.Repo+":"+n.Tag, id)
	}
}

// Well-known images from public registries are infrastructure, not code in
// this workspace. Only used to say "external" rather than "not found".
var wellKnownImages = map[string]bool{
	"redis": true, "postgres": true, "mysql": true, "mariadb": true, "mongo": true,
	"mongodb": true, "rabbitmq": true, "kafka": true, "zookeeper": true, "nginx": true,
	"busybox": true, "alpine": true, "ubuntu": true, "debian": true, "memcached": true,
	"elasticsearch": true, "opensearch": true, "minio": true, "localstack": true,
	"prometheus": true, "grafana": true, "jaeger": true, "zipkin": true, "consul": true,
	"vault": true, "traefik": true, "envoy": true, "valkey": true, "cassandra": true,
	"etcd": true, "keycloak": true, "mailhog": true, "wiremock": true, "pgvector": true,
	"meilisearch": true, "clickhouse-server": true, "redpanda": true, "cp-kafka": true,
	"otel-collector": true, "opentelemetry-collector-contrib": true, "haproxy": true,
}
