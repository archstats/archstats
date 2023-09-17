package analysis

import (
	"github.com/RyanSusana/archstats/analysis/file"
	"github.com/samber/lo"
)

type StatAccumulatorFunction func(statsToMerge []interface{}) interface{}

type accumulatorIndex struct {
	AccumulateFunctions map[string]StatAccumulatorFunction
}

func (merger *accumulatorIndex) getAccumulatorFunction(statType string) StatAccumulatorFunction {
	function, exists := merger.AccumulateFunctions[statType]
	if exists {
		return function
	}
	return SumStatMerger
}

func (merger *accumulatorIndex) merge(statsToMerge []*file.StatRecord) *file.Stats {
	statsToReturn := make(file.Stats)

	nonNilStats := lo.Filter(statsToMerge, func(stats *file.StatRecord, _ int) bool {
		return stats != nil
	})

	groupedStats := lo.GroupBy(nonNilStats, func(stat *file.StatRecord) string {
		return stat.StatType
	})

	for statType, records := range groupedStats {
		function := merger.getAccumulatorFunction(statType)
		recordValues := lo.Map(records, func(record *file.StatRecord, _ int) interface{} {
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
