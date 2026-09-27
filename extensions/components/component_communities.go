package components

import (
	"github.com/archstats/archstats/core"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/community"
)

func CommunitiesView(results *core.Results) (view *core.View) {
	theGraph := results.ComponentGraph

	if theGraph == nil || theGraph.Nodes().Len() < 3 || !hasEdges(theGraph) {
		// With no edges there is nothing to group by, and gonum's directed
		// Louvain indexes community -1 on such a graph and panics: the whole
		// export of spring-petclinic-microservices died on it.
		return core.CreateViewFromRows("component_communities", nil)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Warn().Msgf("component communities could not be computed: %v", r)
			view = core.CreateViewFromRows("component_communities", nil)
		}
	}()

	louvain := community.Modularize(theGraph, 1.0, nil)

	var rows []*core.Row

	for i, theCommunity := range louvain.Communities() {
		theCommunityComponentNames := lo.Map(theCommunity, func(node graph.Node, _ int) string {
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
			row.Data["community_nr"] = i
			row.Data["community_size"] = len(theCommunity)

			setGraphMetricsOnRowWithPrefix(row, metrics, "community__")
			rows = append(rows, row)
		}
	}
	return core.CreateViewFromRows("component_communities", rows)
}

func hasEdges(g graph.Directed) bool {
	nodes := g.Nodes()
	for nodes.Next() {
		if g.From(nodes.Node().ID()).Len() > 0 {
			return true
		}
	}
	return false
}
