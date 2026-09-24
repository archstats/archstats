package basic

import (
	"testing"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
)

func TestUnitUsesListsEachModuleOnce(t *testing.T) {
	results := &core.Results{Units: []*unit.Unit{
		{ID: "TestBSON", Refs: []unit.Ref{
			{Module: "go.mongodb.org/mongo-driver/v2/bson", Name: "M"},
			{Module: "go.mongodb.org/mongo-driver/v2/bson", Name: "Marshal"},
			{Module: "net/http", Name: "Get"},
			{Name: "local"},
		}},
		{ID: "TestRender", Refs: []unit.Ref{{Module: "net/http", Name: "Get"}}},
	}}
	var got []string
	for _, r := range unitUsesView(results).Rows {
		got = append(got, r.Data["unit"].(string)+" "+r.Data["module"].(string))
	}
	assert.Equal(t, []string{
		"TestBSON go.mongodb.org/mongo-driver/v2/bson",
		"TestBSON net/http",
		"TestRender net/http",
	}, got)
}
