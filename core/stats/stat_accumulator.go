package stats

import (
	"github.com/samber/lo"
)

type StatAccumulatorFunction func(statsToMerge []interface{}) interface{}

type StatAccumulator struct {
	AccumulateFunctions map[string]StatAccumulatorFunction
}

func (merger *StatAccumulator) getAccumulatorFunction(statType string) StatAccumulatorFunction {
	function, exists := merger.AccumulateFunctions[statType]
	if exists {
		return function
	}
	return SumStatMerger
}

func (merger *StatAccumulator) Merge(statsToMerge []*Record) *Stats {
	statsToReturn := make(Stats)

	nonNilStats := lo.Filter(statsToMerge, func(stats *Record, _ int) bool {
		return stats != nil
	})

	groupedStats := lo.GroupBy(nonNilStats, func(stat *Record) string {
		return stat.StatType
	})

	for statType, records := range groupedStats {
		function := merger.getAccumulatorFunction(statType)
		recordValues := lo.Map(records, func(record *Record, _ int) interface{} {
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

func LastRecordStatMerger(thingsToMerge []interface{}) interface{} {
	if len(thingsToMerge) == 0 {
		return nil
	}
	return thingsToMerge[len(thingsToMerge)-1]
}

func MostCommonStatMerger(thingsToMerge []interface{}) interface{} {
	countMap := make(map[interface{}]int)
	type returnT struct {
		thing interface{}
		count int
	}
	for _, thing := range thingsToMerge {
		countMap[thing] += 1
	}

	results := lo.MapToSlice(countMap, func(key interface{}, count int) *returnT {

		return &returnT{
			thing: key,
			count: count,
		}
	})

	mostCommon := lo.Reduce(results, func(agg *returnT, item *returnT, index int) *returnT {
		if agg == nil || item.count > agg.count {
			return item
		}
		return agg
	}, nil)
	return mostCommon.thing

}
