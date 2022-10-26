package analysis

import (
	_ "embed"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"gonum.org/v1/gonum/graph/topo"
	"sort"
	"strings"
	"testing"
)

//go:embed elepy_connections.txt
var elepyConnections string

func TestElepy(t *testing.T) {
	input := connectionStringsToResults(strings.Split(elepyConnections, "\n"))

	theGraph := createComponentGraph(input.Connections)
	cycles := topo.DirectedCyclesIn(theGraph)

	sort.Slice(cycles, func(i, j int) bool {
		return len(cycles[i]) > len(cycles[j])
	})
	assert.Len(t, cycles[0], 17)
}
func TestGraphCreation(t *testing.T) {
	input := connectionStringsToResults([]string{
		"A -> B",
		"B -> C",
		"C -> D",
		"D -> A",
	})

	theGraph := createComponentGraph(input.Connections)

	shouldBeC := theGraph.To(theGraph.ComponentToId("A"))
	shouldBeC.Next()
	assert.Equal(t, "D", shouldBeC.Node().(*componentNode).name)

	shouldBeB := theGraph.From(theGraph.ComponentToId("A"))
	shouldBeB.Next()
	assert.Equal(t, "B", shouldBeB.Node().(*componentNode).name)

	assert.Equal(t, 4, theGraph.Nodes().Len())

	assert.True(t, theGraph.HasEdgeBetween(theGraph.ComponentToId("A"), theGraph.ComponentToId("B")), "should have edge between A & B")
	assert.True(t, theGraph.HasEdgeBetween(theGraph.ComponentToId("B"), theGraph.ComponentToId("A")), "should have edge between B & A")
	assert.False(t, theGraph.HasEdgeBetween(theGraph.ComponentToId("A"), theGraph.ComponentToId("C")), "should not have edge between A & C")
	assert.False(t, theGraph.HasEdgeBetween(theGraph.ComponentToId("D"), theGraph.ComponentToId("B")), "should not have edge between D & B")

	assert.True(t, theGraph.HasEdgeFromTo(theGraph.ComponentToId("A"), theGraph.ComponentToId("B")), "A -> B")
	assert.False(t, theGraph.HasEdgeFromTo(theGraph.ComponentToId("B"), theGraph.ComponentToId("A")), "B -> A")

	cycles := topo.DirectedCyclesIn(theGraph)

	assert.Len(t, cycles, 1)
}

func connectionStringsToResults(inputs []string) *Results {
	connections := make([]*ComponentConnection, 0, len(inputs))
	components := make(map[string][]*Snippet, 0)

	for _, input := range inputs {
		connection := splitInput(input)
		components[connection.From] = []*Snippet{}
		components[connection.To] = []*Snippet{}
		connections = append(connections, connection)
	}

	return &Results{
		SnippetsByComponent: components,
		Connections:         connections,
		ConnectionsFrom: lo.GroupBy(connections, func(connection *ComponentConnection) string {
			return connection.From
		}),
		ConnectionsTo: lo.GroupBy(connections, func(connection *ComponentConnection) string {
			return connection.To
		}),
	}
}

func splitInput(input string) *ComponentConnection {
	split := strings.Split(input, "->")
	connection := &ComponentConnection{
		From: strings.TrimSpace(split[0]),
		To:   strings.TrimSpace(split[1]),
	}
	return connection
}
