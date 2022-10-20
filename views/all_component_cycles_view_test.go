package views

import (
	_ "embed"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"sort"
	"strconv"
	"testing"
)

func TestNoCycles(t *testing.T) {
	input := connectionStringsToResults([]string{
		"A -> B",
		"A -> C",
		"B -> D",
	})

	results := ComponentCyclesView(input).Rows
	assert.Len(t, results, 0)
	assertNotPartOfCycle(t, results, "A", "B", "C", "D")
}

func TestOneSimpleCycle(t *testing.T) {
	input := connectionStringsToResults([]string{
		"A -> B",
		"B -> A",
	})

	results := ComponentCyclesView(input).Rows
	assert.Len(t, results, 2)
}

func TestOneComplexCycle(t *testing.T) {
	input := connectionStringsToResults([]string{
		"A -> B",
		"B -> C",
		"C -> D",
		"D -> A",
	})

	results := ComponentCyclesView(input).Rows

	index := lo.GroupBy(results, func(row *Row) string {
		return row.Data["component"].(string)
	})

	expectedCycle := []string{"D", "C", "B", "A"}

	assert.Len(t, results, 4)
	assertHasCycle(t, results, expectedCycle)

	for _, componentName := range expectedCycle {
		componentRow := index[componentName][0]
		assert.Equal(t, input.ConnectionsFrom[componentName][0].To, componentRow.Data["successor"])
		assert.Equal(t, input.ConnectionsTo[componentName][0].From, componentRow.Data["predecessor"])
	}
}

func TestTwoSeparateCycles(t *testing.T) {
	input := connectionStringsToResults([]string{
		"A -> B",
		"B -> C",
		"C -> A",
		"X -> Y",
		"Y -> Z",
		"Z -> X",
		"L -> M",
		"M -> N",
		"N -> O",
		"O -> P",
	})

	results := ComponentCyclesView(input).Rows
	assert.Len(t, results, 6)
	assertHasCycle(t, results, []string{"A", "B", "C"})
	assertHasCycle(t, results, []string{"X", "Y", "Z"})
	assertNotPartOfCycle(t, results, "L", "M", "N", "O", "P")
}

func TestTwoJoinedCycles(t *testing.T) {
	input := connectionStringsToResults([]string{
		"JOIN -> B",
		"B -> C",
		"C -> JOIN",
		"JOIN -> X",
		"X -> Y",
		"Y -> Z",
		"Z -> JOIN",
	})

	results := ComponentCyclesView(input).Rows
	assert.Len(t, results, 7)
	assertHasCycle(t, results, []string{"B", "C", "JOIN"})
	assertHasCycle(t, results, []string{"X", "Y", "Z", "JOIN"})
}
func assertNotPartOfCycle(t *testing.T, results []*Row, component ...string) {
	index := lo.GroupBy(results, func(row *Row) string {
		return row.Data["component"].(string)
	})

	for _, component := range component {
		if index[component] != nil {
			assert.Fail(t, "component should not be part of a cycle", component)
		}
	}
}

func assertHasCycle(t *testing.T, results []*Row, expectedCycle []string) {
	grouped := lo.GroupBy(results, func(row *Row) string {
		return strconv.Itoa(row.Data["cycle_nr"].(int))
	})

	for _, theCycle := range grouped {
		components := mapTo(theCycle, func(row *Row) string {
			return row.Data["component"].(string)
		})
		if cycleMatches(components, expectedCycle) {
			return
		}
	}

	var formattedGroups [][]string
	for _, theCycle := range grouped {
		formattedGroups = append(formattedGroups, mapTo(theCycle, func(row *Row) string {
			return row.Data["component"].(string)
		}))
	}
	assert.Failf(t, "missing cycle", "expected cycle %v not found. All cycles: %v", expectedCycle, formattedGroups)
}

func cycleMatches(i []string, expectedCycle []string) bool {
	if len(i) != len(expectedCycle) {
		return false
	}
	sort.Strings(i)
	sort.Strings(expectedCycle)
	for index, item := range i {
		if item != expectedCycle[index] {
			return false
		}
	}
	return true
}
