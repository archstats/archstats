package views

import (
	"archstats/snippets"
	"math"
)

func ComponentView(results *snippets.Results) *View {
	view := GenericView(getDistinctColumnsFromResults(results), results.SnippetsByComponent)

	for _, row := range view.Rows {
		component := row.Data["name"].(string)
		afferentCouplings, efferentCouplings := len(results.ConnectionsTo[component]), len(results.ConnectionsFrom[component])
		abstractness := row.Data["abstractness"].(float64)
		instability := math.Max(0, math.Min(1, float64(efferentCouplings)/float64(afferentCouplings+efferentCouplings)))
		distanceMainSequence := abstractness + instability - 1

		row.Data[AfferentCouplings] = afferentCouplings
		row.Data[EfferentCouplings] = efferentCouplings
		row.Data[Instability] = instability
		row.Data[DistanceMainSequence] = distanceMainSequence
	}
	view.OrderedColumns = append(view.OrderedColumns, AfferentCouplings, EfferentCouplings, Instability)

	return view
}