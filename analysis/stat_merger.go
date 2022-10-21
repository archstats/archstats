package analysis

import "github.com/samber/lo"

type StatMergeFunction func(statsToMerge []interface{}) interface{}

type merger struct {
	MergeFunctions map[string]StatMergeFunction
}

func (merger *merger) getMergerFunction(statType string) StatMergeFunction {
	function, exists := merger.MergeFunctions[statType]
	if exists {
		return function
	}
	return SumStatMerger
}

func (merger *merger) merge(statsToMerge []*StatRecord) *Stats {
	statsToReturn := make(Stats)

	nonNilStats := lo.Filter(statsToMerge, func(stats *StatRecord, _ int) bool {
		return stats != nil
	})

	groupedStats := lo.GroupBy(nonNilStats, func(stat *StatRecord) string {
		return stat.StatType
	})

	for statType, records := range groupedStats {
		function := merger.getMergerFunction(statType)
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
