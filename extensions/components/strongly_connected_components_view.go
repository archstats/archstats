package components

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/topo"
)

func StronglyConnectedView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	groups := topo.TarjanSCC(theGraph)
	var rows []*core.Row
	for groupNr, theGroup := range groups {
		theCommunityComponentNames := lo.Map(theGroup, func(node graph.Node, _ int) string {
			return theGraph.IdToComponent(node.ID())
		})

		communitySubgraph := createSubGraph(theCommunityComponentNames, theGraph)
		metricsIndex := createComponentInGraphMetrics(communitySubgraph)
		for _, component := range communitySubgraph.Components {
			row := &core.Row{
				Data: map[string]interface{}{},
			}

			metrics := metricsIndex[component]
			row.Data["component"] = component
			row.Data["group"] = groupNr
			row.Data["group_size"] = len(theGroup)

			setGraphMetricsOnRowWithPrefix(row, metrics, "group__")
			rows = append(rows, row)
		}
	}
	return core.CreateViewFromRows("strongly_connected_components", rows)
}
