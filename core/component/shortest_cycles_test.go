package component

import (
	_ "embed"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		cycle, expected string
	}{
		{cycle: "A -> B -> C", expected: "A -> B -> C"},
		{cycle: "B -> C -> A", expected: "A -> B -> C"},
		{cycle: "C -> A -> B", expected: "A -> B -> C"},

		{cycle: "A -> C -> B", expected: "A -> C -> B"},
		{cycle: "B -> A -> C", expected: "A -> C -> B"},
		{cycle: "C -> B -> A", expected: "A -> C -> B"},
	}
	for _, test := range tests {
		t.Run(test.cycle, func(t *testing.T) {

			splittedCycle := strings.Split(test.cycle, " -> ")
			actual := normalizeCycle(splittedCycle)

			joinedExpectedCycle := strings.Join(actual, " -> ")
			assert.Equal(t, test.expected, joinedExpectedCycle)
		})
	}
}

func TestShortestCycles(t *testing.T) {
	input := []string{
		"PA -> PB",
		"PB -> PA",
		"PB -> PC",
		"PB -> PD",
		"PD -> PB",
		"PD -> PE",
		"PE -> PA",
		"PC -> PD",
		"PC -> PF",
		"PF -> PG",
		"PG -> PH",
		"PH -> PG",
	}

	theGraph := CreateGraph(connectionStringsToConnections(input))
	actualCycles := shortestCycles(theGraph)

	expectedCycles := []string{
		"PA -> PB -> PD -> PE -> PA",
		"PB -> PD -> PB",
		"PB -> PC -> PD -> PB",
		"PA -> PB -> PA",
		"PG -> PH -> PG",
	}

	elementaryCycleNotToBeExpected := "PA -> PB -> PC -> PD -> PE -> PA"

	assert.Len(t, actualCycles, len(expectedCycles))
	actualCyclesKeys := lo.Keys(actualCycles)
	assert.ElementsMatch(t, actualCyclesKeys, expectedCycles)
	assert.NotContainsf(t, actualCyclesKeys, elementaryCycleNotToBeExpected, "should not contain elementary cycle '%s'", elementaryCycleNotToBeExpected)
	assertShortestCyclesCorrectness(t, theGraph, actualCycles)
}

//go:embed shortest_cycles_mockito_test.txt
var mockito string

func TestGraph_ShortestCycles(t *testing.T) {
	mockitoLines := strings.Split(mockito, "\n")

	theGraph := CreateGraph(connectionStringsToConnections(mockitoLines))

	cycles := theGraph.ShortestCycles()

	assertShortestCyclesCorrectness(t, theGraph, cycles)
}

func assertShortestCyclesCorrectness(t *testing.T, graph *Graph, cycles map[string]Cycle) {
	for key, cycle := range cycles {
		pairs := splitIntoPairs(cycle)
		for _, pair := range pairs {

			from, to := pair[0], pair[1]
			fromId, toId := graph.ComponentToId(from), graph.ComponentToId(to)

			assert.True(t, graph.HasEdgeFromTo(fromId, toId), "graph should have edge from '%s' to '%s' to fulfill '%s'", from, to, key)
		}
	}
}
