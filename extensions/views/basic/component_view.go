package basic

import (
	"github.com/RyanSusana/archstats/analysis"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/path"
	"math"
)

const (
	Name                 = "name"
	AfferentCouplings    = "afferent_couplings"
	EfferentCouplings    = "efferent_couplings"
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
)

func componentView(results *analysis.Results) *analysis.View {
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
		component := row.Data["name"].(string)
		componentId := graph.ComponentToId(component)

		afferentCouplings, efferentCouplings := len(results.ConnectionsTo[component]), len(results.ConnectionsFrom[component])
		abstractness := convertToFloat(row.Data["abstractness"])
		instability := math.Max(0, math.Min(1, float64(efferentCouplings)/float64(afferentCouplings+efferentCouplings)))
		distanceMainSequence := math.Abs(abstractness + instability - 1)

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
		analysis.IntColumn(AfferentCouplings),
		analysis.IntColumn(EfferentCouplings),
		analysis.FloatColumn(Instability),
		analysis.FloatColumn(DistanceMainSequence),
		analysis.FloatColumn(Betweenness),
		analysis.FloatColumn(PageRank),
		analysis.FloatColumn(HubScore),
		analysis.FloatColumn(AuthorityScore),
		analysis.FloatColumn(HarmonicCentrality),
		analysis.FloatColumn(FarnessCentrality),
		analysis.FloatColumn(ResidualCloseness),
	)

	return view
}

func convertToFloat(input interface{}) float64 {
	if input == nil {
		return 0
	}
	return input.(float64)
}
