package basic

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/path"
	"math"
)

const (
	Name                 = "name"
	AfferentCouplings    = "afferent_coupling_count"
	EfferentCouplings    = "efferent_coupling_count"
	Instability          = "instability"
	Abstractness         = "abstractness"
	DistanceMainSequence = "distance_main_sequence"
	Betweenness          = "betweenness"
	PageRank             = "page_rank"
	HubScore             = "hub_score"
	AuthorityScore       = "authority_score"
	HarmonicCentrality   = "harmonic_centrality"
	FarnessCentrality    = "farness_centrality"
	ResidualCloseness    = "residual_closeness"
	Dependents           = "direct_dependent_count"
	Dependencies         = "direct_dependency_count"
)

func componentView(results *core.Results) *core.View {
	view := genericView(getDistinctColumnsFromResults(results), results.StatsByComponent)

	graph := results.ComponentGraph
	allShortestPaths := path.DijkstraAllPaths(graph)
	betweennessIndex := network.Betweenness(graph)
	pageRankIndex := network.PageRank(results.ComponentGraph, 0.85, 0.00001)
	hubAuthorityHITSIndex := network.HITS(results.ComponentGraph, 0.00001)

	harmonicCentralityIndex := network.Harmonic(results.ComponentGraph, allShortestPaths)
	farnessCentralityIndex := network.Farness(results.ComponentGraph, allShortestPaths)
	residualClosenessIndex := network.Residual(results.ComponentGraph, allShortestPaths)

	for _, row := range view.Rows {
		componentName := row.Data["name"].(string)
		componentId := graph.ComponentToId(componentName)

		afferentCouplings, efferentCouplings := countUniqueFilesInConnections(results.ConnectionsTo[componentName]), countUniqueFilesInConnections(results.ConnectionsFrom[componentName])
		abstractness := convertToFloat(row.Data["abstractness"])
		instability := math.Max(0, math.Min(1, float64(efferentCouplings)/float64(afferentCouplings+efferentCouplings)))
		distanceMainSequence := math.Abs(nanToZero(abstractness) + nanToZero(instability) - 1)

		row.Data[Dependents] = len(lo.UniqBy(results.ConnectionsTo[componentName], func(item *component.Connection) string {
			return item.From
		}))
		row.Data[Dependencies] = len(lo.UniqBy(results.ConnectionsTo[componentName], func(item *component.Connection) string {
			return item.To
		}))
		row.Data[AfferentCouplings] = afferentCouplings
		row.Data[EfferentCouplings] = efferentCouplings
		row.Data[Instability] = nanToZero(instability)
		row.Data[DistanceMainSequence] = nanToZero(distanceMainSequence)
		row.Data[Betweenness] = betweennessIndex[componentId]
		row.Data[PageRank] = pageRankIndex[componentId]
		row.Data[HubScore] = hubAuthorityHITSIndex[componentId].Hub
		row.Data[AuthorityScore] = hubAuthorityHITSIndex[componentId].Authority
		row.Data[HarmonicCentrality] = harmonicCentralityIndex[componentId]
		row.Data[FarnessCentrality] = farnessCentralityIndex[componentId]
		row.Data[ResidualCloseness] = residualClosenessIndex[componentId]
	}
	view.Columns = append(view.Columns,
		core.IntColumn(Dependents),
		core.IntColumn(Dependencies),
		core.IntColumn(AfferentCouplings),
		core.IntColumn(EfferentCouplings),
		core.FloatColumn(Instability),
		core.FloatColumn(DistanceMainSequence),
		core.FloatColumn(Betweenness),
		core.FloatColumn(PageRank),
		core.FloatColumn(HubScore),
		core.FloatColumn(AuthorityScore),
		core.FloatColumn(HarmonicCentrality),
		core.FloatColumn(FarnessCentrality),
		core.FloatColumn(ResidualCloseness),
	)

	return view
}

func countUniqueFilesInConnections(connections []*component.Connection) int {
	uniqueFiles := make(map[string]bool)
	for _, connection := range connections {
		uniqueFiles[connection.File] = true
	}
	return len(uniqueFiles)
}

func convertToFloat(input interface{}) float64 {
	if input == nil {
		return 0
	}
	return input.(float64)
}
