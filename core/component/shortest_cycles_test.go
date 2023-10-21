package component

import (
	_ "embed"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
}
