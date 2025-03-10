package components

import (
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/extensions/util"
	"math"
)

const (
	AfferentCouplings    = "modularity__coupling__afferent"
	EfferentCouplings    = "modularity__coupling__efferent"
	Dependents           = "modularity__coupling__dependents"
	Dependencies         = "modularity__coupling__dependencies"
	Instability          = "modularity__instability"
	Abstractness         = "modularity__abstractness"
	DistanceMainSequence = "modularity__distance_main_sequence"
	Betweenness          = "graph__betweenness"
	PageRank             = "graph__page_rank"
	HubScore             = "graph__hits__hub_score"
	AuthorityScore       = "graph__hits__authority_score"
	HarmonicCentrality   = "graph__harmonic_centrality"
	FarnessCentrality    = "graph__farness_centrality"
	ResidualCloseness    = "graph__residual_closeness"
	ShortCycleCount      = "cycles__short__count"
	ShortCycleSizeAvg    = "cycles__short__avg"
	ShortCycleSizeMax    = "cycles__short__max"
)

func MainView(results *core.Results) *core.View {
	view := util.GenericView(util.GetDistinctColumnsFrom(results.StatsByComponent), results.StatsByComponent)

	view.Columns = append(view.Columns,
		core.IntColumn(Dependents),
		core.IntColumn(Dependencies),
		core.IntColumn(AfferentCouplings),
		core.IntColumn(EfferentCouplings),
		core.FloatColumn(Abstractness),
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

	componentInGraphMetricsIndex := createComponentInGraphMetrics(graph)
	for _, row := range view.Rows {
		componentName := row.Data[util.Name].(string)
		abstractness := calculateAbstractness(row)
		componentInGraphMetrics := componentInGraphMetricsIndex[componentName]
		if componentInGraphMetrics == nil {
			continue
		}
		efferentCouplings, afferentCouplings := componentInGraphMetrics.EfferentCouplings, componentInGraphMetrics.AfferentCouplings
		instability := math.Max(0, math.Min(1, float64(efferentCouplings)/float64(afferentCouplings+efferentCouplings)))
		distanceMainSequence := math.Abs(util.NanToZero(abstractness) + util.NanToZero(instability) - 1)
		row.Data[Instability] = util.NanToZero(instability)
		row.Data[Abstractness] = abstractness
		row.Data[DistanceMainSequence] = util.NanToZero(distanceMainSequence)

		setGraphMetricsOnRow(row, componentInGraphMetrics)
	}

	return view
}

func calculateAbstractness(row *core.Row) float64 {
	abstractTypes := util.ToInt(row.Data[file.AbstractType])
	types := util.ToInt(row.Data[file.Type])
	abstractness := math.Max(0, math.Min(1, float64(abstractTypes)/float64(types)))
	return util.NanToZero(abstractness)
}
