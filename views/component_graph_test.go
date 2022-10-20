package views

import (
	_ "embed"
	"github.com/RyanSusana/archstats/analysis"
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

	theGraph := createGraph(input)
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

	theGraph := createGraph(input)

	shouldBeC := theGraph.To(theGraph.ComponentId("A"))
	shouldBeC.Next()
	assert.Equal(t, "D", shouldBeC.Node().(*componentNode).name)

	shouldBeB := theGraph.From(theGraph.ComponentId("A"))
	shouldBeB.Next()
	assert.Equal(t, "B", shouldBeB.Node().(*componentNode).name)

	assert.Equal(t, 4, theGraph.Nodes().Len())

	assert.True(t, theGraph.HasEdgeBetween(theGraph.ComponentId("A"), theGraph.ComponentId("B")), "should have edge between A & B")
	assert.True(t, theGraph.HasEdgeBetween(theGraph.ComponentId("B"), theGraph.ComponentId("A")), "should have edge between B & A")
	assert.False(t, theGraph.HasEdgeBetween(theGraph.ComponentId("A"), theGraph.ComponentId("C")), "should not have edge between A & C")
	assert.False(t, theGraph.HasEdgeBetween(theGraph.ComponentId("D"), theGraph.ComponentId("B")), "should not have edge between D & B")

	assert.True(t, theGraph.HasEdgeFromTo(theGraph.ComponentId("A"), theGraph.ComponentId("B")), "A -> B")
	assert.False(t, theGraph.HasEdgeFromTo(theGraph.ComponentId("B"), theGraph.ComponentId("A")), "B -> A")

	cycles := topo.DirectedCyclesIn(theGraph)

	assert.Len(t, cycles, 1)
}

func connectionStringsToResults(inputs []string) *analysis.Results {
	connections := make([]*analysis.ComponentConnection, 0, len(inputs))
	components := make(map[string][]*analysis.Snippet, 0)

	for _, input := range inputs {
		connection := splitInput(input)
		components[connection.From] = []*analysis.Snippet{}
		components[connection.To] = []*analysis.Snippet{}
		connections = append(connections, connection)
	}

	return &analysis.Results{
		SnippetsByComponent: components,
		Connections:         connections,
		ConnectionsFrom: groupBy(connections, func(connection *analysis.ComponentConnection) string {
			return connection.From
		}),
		ConnectionsTo: groupBy(connections, func(connection *analysis.ComponentConnection) string {
			return connection.To
		}),
	}
}

func splitInput(input string) *analysis.ComponentConnection {
	split := strings.Split(input, "->")
	connection := &analysis.ComponentConnection{
		From: strings.TrimSpace(split[0]),
		To:   strings.TrimSpace(split[1]),
	}
	return connection
}
