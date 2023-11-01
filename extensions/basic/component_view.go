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
	AfferentCouplings    = "modularity:coupling:afferent"
	EfferentCouplings    = "modularity:coupling:efferent"
	Dependents           = "modularity:coupling:dependents"
	Dependencies         = "modularity:coupling:dependencies"
	Instability          = "modularity:instability"
	Abstractness         = "modularity:abstractness"
	DistanceMainSequence = "modularity:distance_main_sequence"
	Betweenness          = "graph:betweenness"
	PageRank             = "graph:page_rank"
	HubScore             = "graph:hits:hub_score"
	AuthorityScore       = "graph:hits:authority_score"
	HarmonicCentrality   = "graph:harmonic_centrality"
	FarnessCentrality    = "graph:farness_centrality"
	ResidualCloseness    = "graph:residual_closeness"

	ShortCycleCount   = "cycles:short:count"
	ShortCycleSizeAvg = "cycles:short:avg"
	ShortCycleSizeMax = "cycles:short:max"
)

func componentView(results *core.Results) *core.View {
	view := genericView(getDistinctColumnsFrom(results.StatsByComponent), results.StatsByComponent)

	cyclesPerComponent := cyclesPerComponent(results.ComponentGraph.ShortestCycles())
	view.Columns = append(view.Columns,
		core.IntColumn(Dependents),
		core.IntColumn(Dependencies),
		core.IntColumn(AfferentCouplings),
		core.IntColumn(EfferentCouplings),
		core.FloatColumn(Instability),
		core.FloatColumn(DistanceMainSequence),
		core.IntColumn(ShortCycleCount),
		core.FloatColumn(ShortCycleSizeAvg),
		core.IntColumn(ShortCycleSizeMax),
		core.FloatColumn(Betweenness),
		core.FloatColumn(PageRank),
		core.FloatColumn(HubScore),
		core.FloatColumn(AuthorityScore),
		core.FloatColumn(HarmonicCentrality),
		core.FloatColumn(FarnessCentrality),
		core.FloatColumn(ResidualCloseness),
	)

	graph := results.ComponentGraph

	if len(graph.Components) == 0 {
		return view
	}

	allShortestPaths := path.DijkstraAllPaths(graph)
	betweennessIndex := network.Betweenness(graph)
	pageRankIndex := network.PageRank(results.ComponentGraph, 0.85, 0.00001)
	hubAuthorityHITSIndex := network.HITS(results.ComponentGraph, 0.00001)

	harmonicCentralityIndex := network.Harmonic(results.ComponentGraph, allShortestPaths)
	farnessCentralityIndex := network.Farness(results.ComponentGraph, allShortestPaths)
	residualClosenessIndex := network.Residual(results.ComponentGraph, allShortestPaths)

	for _, row := range view.Rows {
		componentName := row.Data[Name].(string)
		componentId := graph.ComponentToId(componentName)

		afferentCouplings, efferentCouplings := countUniqueFilesInConnections(results.ConnectionsTo[componentName]), countUniqueFilesInConnections(results.ConnectionsFrom[componentName])
		abstractness := convertToFloat(row.Data[Abstractness])
		instability := math.Max(0, math.Min(1, float64(efferentCouplings)/float64(afferentCouplings+efferentCouplings)))
		distanceMainSequence := math.Abs(nanToZero(abstractness) + nanToZero(instability) - 1)

		row.Data[Dependents] = len(lo.UniqBy(results.ConnectionsTo[componentName], func(item *component.Connection) string {
			return item.From
		}))
		row.Data[Dependencies] = len(lo.UniqBy(results.ConnectionsFrom[componentName], func(item *component.Connection) string {
			return item.To
		}))
		cycleAvg, cycleMax := avgMaxCycleLength(cyclesPerComponent[componentName])
		row.Data[ShortCycleCount] = len(cyclesPerComponent[componentName])
		row.Data[ShortCycleSizeAvg] = cycleAvg
		row.Data[ShortCycleSizeMax] = cycleMax
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

	return view
}

func cyclesPerComponent(allCycles map[string]component.Cycle) map[string][]component.Cycle {
	cyclesPerComponent := make(map[string][]component.Cycle)
	for _, cycle := range allCycles {
		for _, cmpnt := range cycle {
			cyclesPerComponent[cmpnt] = append(cyclesPerComponent[cmpnt], cycle)
		}
	}
	return cyclesPerComponent
}

func avgMaxCycleLength(all []component.Cycle) (float64, int) {
	if len(all) == 0 {
		return 0, 0
	}
	maxCycle := 0
	var sum float64
	for _, cycle := range all {
		// subtract 1 because the last element is the same as the first
		sum += float64(len(cycle) - 1)
		if len(cycle)-1 > maxCycle {
			maxCycle = len(cycle) - 1
		}
	}

	return sum / float64(len(all)), maxCycle
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
