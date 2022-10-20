package views

import (
	"github.com/RyanSusana/archstats/analysis"
	"math"
)

const (
	Name                 = "name"
	AfferentCouplings    = "afferent_couplings"
	EfferentCouplings    = "efferent_couplings"
	Instability          = "instability"
	Abstractness         = "abstractness"
	DistanceMainSequence = "distance_main_sequence"
)

func ComponentView(results *analysis.Results) *View {
	view := GenericView(getDistinctColumnsFromResults(results), results.StatsByComponent)

	for _, row := range view.Rows {
		component := row.Data["name"].(string)
		afferentCouplings, efferentCouplings := len(results.ConnectionsTo[component]), len(results.ConnectionsFrom[component])
		abstractness := convertToFloat(row.Data["abstractness"])
		instability := math.Max(0, math.Min(1, float64(efferentCouplings)/float64(afferentCouplings+efferentCouplings)))
		distanceMainSequence := math.Abs(abstractness + instability - 1)

		row.Data[AfferentCouplings] = afferentCouplings
		row.Data[EfferentCouplings] = efferentCouplings
		row.Data[Instability] = nanToZero(instability)
		row.Data[DistanceMainSequence] = nanToZero(distanceMainSequence)
	}
	view.Columns = append(view.Columns,
		IntColumn(AfferentCouplings),
		IntColumn(EfferentCouplings),
		FloatColumn(Instability),
		FloatColumn(DistanceMainSequence),
	)

	return view
}

func convertToFloat(input interface{}) float64 {
	if input == nil {
		return 0
	}
	return input.(float64)
}
