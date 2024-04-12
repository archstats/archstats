package components

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/path"
)

type ComponentInGraphMetrics struct {
	Name               string
	AfferentCoupling   int
	EfferentCoupling   int
	Betweenness        float64
	ShortCycleCount    int
	PageRank           float64
	ShortCycleSizeAvg  float64
	ShortCycleSizeMax  int
	AfferentCouplings  int
	EfferentCouplings  int
	HubScore           float64
	AuthorityScore     float64
	HarmonicCentrality float64
	FarnessCentrality  float64
	ResidualCloseness  float64
	Dependencies       int
	Dependents         int
}

func createComponentInGraphMetrics(graph *component.Graph) map[string]*ComponentInGraphMetrics {
	toReturn := make(map[string]*ComponentInGraphMetrics)
	allShortestPaths := path.DijkstraAllPaths(graph)
	cyclesPerComponent := cyclesPerComponent(graph.ShortestCycles())
	betweennessIndex := network.Betweenness(graph)
	pageRankIndex := network.PageRank(graph, 0.85, 0.00001)
	var hubAuthorityHITSIndex map[int64]network.HubAuthority
	if !graph.NoConnections() {
		hubAuthorityHITSIndex = network.HITS(graph, 0.00001)
	} else {
		hubAuthorityHITSIndex = make(map[int64]network.HubAuthority)
		for _, componentName := range graph.Components {
			componentId := graph.ComponentToId(componentName)
			hubAuthorityHITSIndex[componentId] = network.HubAuthority{Hub: 0, Authority: 0}
		}
	}

	harmonicCentralityIndex := network.Harmonic(graph, allShortestPaths)
	farnessCentralityIndex := network.Farness(graph, allShortestPaths)
	residualClosenessIndex := network.Residual(graph, allShortestPaths)
	for _, componentName := range graph.Components {
		metrics := &ComponentInGraphMetrics{}
		componentId := graph.ComponentToId(componentName)
		afferentCouplings, efferentCouplings := countUniqueFilesInConnections(graph.ConnectionsTo[componentName]), countUniqueFilesInConnections(graph.ConnectionsFrom[componentName])

		metrics.Dependents = len(lo.UniqBy(graph.ConnectionsTo[componentName], func(item *component.Connection) string {
			return item.From
		}))
		metrics.Dependencies = len(lo.UniqBy(graph.ConnectionsFrom[componentName], func(item *component.Connection) string {
			return item.To
		}))
		cycleAvg, cycleMax := avgMaxCycleLength(cyclesPerComponent[componentName])
		metrics.ShortCycleCount = len(cyclesPerComponent[componentName])
		metrics.ShortCycleSizeAvg = cycleAvg
		metrics.ShortCycleSizeMax = cycleMax
		metrics.AfferentCouplings = afferentCouplings
		metrics.EfferentCouplings = efferentCouplings
		metrics.Betweenness = betweennessIndex[componentId]
		metrics.PageRank = pageRankIndex[componentId]
		metrics.HubScore = hubAuthorityHITSIndex[componentId].Hub
		metrics.AuthorityScore = hubAuthorityHITSIndex[componentId].Authority
		metrics.HarmonicCentrality = harmonicCentralityIndex[componentId]
		metrics.FarnessCentrality = farnessCentralityIndex[componentId]
		metrics.ResidualCloseness = residualClosenessIndex[componentId]

		toReturn[componentName] = metrics
	}
	return toReturn
}
func createSubGraph(components []string, graph *component.Graph) *component.Graph {
	componentLookup := make(map[string]bool)
	for _, component := range components {
		componentLookup[component] = true
	}
	var connections []*component.Connection
	for _, connection := range graph.Connections {
		fromIsPartOfGroup := componentLookup[connection.From]
		toIsPartOfGroup := componentLookup[connection.To]
		if fromIsPartOfGroup && toIsPartOfGroup {
			connections = append(connections, connection)
		}
	}
	groupGraph := component.CreateGraph(components, connections)
	return groupGraph
}

func setGraphMetricsOnRow(row *core.Row, metrics *ComponentInGraphMetrics) {
	setGraphMetricsOnRowWithPrefix(row, metrics, "")
}
func setGraphMetricsOnRowWithPrefix(row *core.Row, metrics *ComponentInGraphMetrics, prefix string) {
	row.Data[prefix+Dependents] = metrics.Dependents
	row.Data[prefix+Dependencies] = metrics.Dependencies
	row.Data[prefix+ShortCycleCount] = metrics.ShortCycleCount
	row.Data[prefix+ShortCycleSizeAvg] = metrics.ShortCycleSizeAvg
	row.Data[prefix+ShortCycleSizeMax] = metrics.ShortCycleSizeMax
	row.Data[prefix+AfferentCouplings] = metrics.AfferentCouplings
	row.Data[prefix+EfferentCouplings] = metrics.EfferentCouplings
	row.Data[prefix+Betweenness] = metrics.Betweenness
	row.Data[prefix+PageRank] = metrics.PageRank
	row.Data[prefix+HubScore] = metrics.HubScore
	row.Data[prefix+AuthorityScore] = metrics.AuthorityScore
	row.Data[prefix+HarmonicCentrality] = metrics.HarmonicCentrality
	row.Data[prefix+FarnessCentrality] = metrics.FarnessCentrality
	row.Data[prefix+ResidualCloseness] = metrics.ResidualCloseness
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
