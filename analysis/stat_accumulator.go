package analysis

import "github.com/samber/lo"

type StatAccumulateFunction func(statsToMerge []interface{}) interface{}

type accumulator struct {
	AccumulateFunctions map[string]StatAccumulateFunction
}

func (merger *accumulator) getAccumulatorFunction(statType string) StatAccumulateFunction {
	function, exists := merger.AccumulateFunctions[statType]
	if exists {
		return function
	}
	return SumStatMerger
}

func (merger *accumulator) merge(statsToMerge []*StatRecord) *Stats {
	statsToReturn := make(Stats)

	nonNilStats := lo.Filter(statsToMerge, func(stats *StatRecord, _ int) bool {
		return stats != nil
	})

	groupedStats := lo.GroupBy(nonNilStats, func(stat *StatRecord) string {
		return stat.StatType
	})

	for statType, records := range groupedStats {
		function := merger.getAccumulatorFunction(statType)
		recordValues := lo.Map(records, func(record *StatRecord, _ int) interface{} {
			return record.Value
		})
		statsToReturn[statType] = function(recordValues)
	}
	return &statsToReturn
}

func SumStatMerger(numbersToMerge []interface{}) interface{} {
	var toReturn interface{}
	nonNilStats := lo.Filter(numbersToMerge, func(stat interface{}, _ int) bool {
		return stat != nil
	})
	for _, stat := range nonNilStats {
		toReturn = sum(toReturn, stat)
	}
	return toReturn
}

func sum(a, b interface{}) interface{} {
	if a == nil {
		return b
	} else if b == nil {
		return a
	} else {
		switch a.(type) {
		case float64:
			return a.(float64) + b.(float64)
		case int:
			return a.(int) + b.(int)
		default:
			return nil
		}
	}
}
