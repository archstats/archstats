package components

import (
	"strings"
	"testing"

	"github.com/archstats/archstats/core/component"
	"github.com/stretchr/testify/assert"
)

func edges(inputs ...string) []*component.Connection {
	connections := make([]*component.Connection, 0, len(inputs))
	for _, input := range inputs {
		parts := strings.Split(input, " -> ")
		connections = append(connections, &component.Connection{From: parts[0], To: parts[1]})
	}
	return connections
}

// cycles__short__count is a count of cycles, not of positions in cycle paths.
// A Cycle closes its loop by repeating its first element, so counting every
// element gave whichever component a cycle started at twice the true number:
// openadmin.dto reported 106 against 53 real cycles.
func TestShortCycleCountCountsCyclesNotPathPositions(t *testing.T) {
	t.Run("every member of a cycle counts it once", func(t *testing.T) {
		graph := component.CreateGraph("", nil, edges("A -> B", "B -> C", "C -> A", "D -> A"))

		metrics := createComponentInGraphMetrics(graph)

		for _, name := range []string{"A", "B", "C"} {
			assert.Equal(t, 1, metrics[name].ShortCycleCount, "%s should be in exactly one cycle", name)
		}
		assert.Equal(t, 0, metrics["D"].ShortCycleCount, "D is in no cycle")
	})

	t.Run("a mutual pair is one cycle for both sides", func(t *testing.T) {
		graph := component.CreateGraph("", nil, edges("A -> B", "B -> A"))

		metrics := createComponentInGraphMetrics(graph)

		assert.Equal(t, 1, metrics["A"].ShortCycleCount)
		assert.Equal(t, 1, metrics["B"].ShortCycleCount)
	})

	t.Run("a component in two cycles counts both", func(t *testing.T) {
		// A closes two separate loops: A -> B -> A and A -> C -> A.
		graph := component.CreateGraph("", nil, edges("A -> B", "B -> A", "A -> C", "C -> A"))

		metrics := createComponentInGraphMetrics(graph)

		assert.Equal(t, 2, metrics["A"].ShortCycleCount)
		assert.Equal(t, 1, metrics["B"].ShortCycleCount)
		assert.Equal(t, 1, metrics["C"].ShortCycleCount)
	})
}
