package component

import (
	_ "embed"
	"github.com/RyanSusana/archstats/analysis/file"
	"github.com/stretchr/testify/assert"
	"gonum.org/v1/gonum/graph/topo"
	"sort"
	"strings"
	"testing"
)

//go:embed elepy_connections.txt
var elepyConnections string

func TestElepy(t *testing.T) {
	input := connectionStringsToConnections(strings.Split(elepyConnections, "\n"))

	theGraph := CreateGraph(input)
	cycles := topo.DirectedCyclesIn(theGraph)

	sort.Slice(cycles, func(i, j int) bool {
		return len(cycles[i]) > len(cycles[j])
	})
	assert.Len(t, cycles[0], 17)
}
func TestGraphCreation(t *testing.T) {
	input := connectionStringsToConnections([]string{
		"A -> B",
		"B -> C",
		"C -> D",
		"D -> A",
	})

	theGraph := CreateGraph(input)

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

func connectionStringsToConnections(inputs []string) []*Connection {
	connections := make([]*Connection, 0, len(inputs))
	components := make(map[string][]*file.Snippet, 0)

	for _, input := range inputs {
		connection := splitInput(input)
		components[connection.From] = []*file.Snippet{}
		components[connection.To] = []*file.Snippet{}
		connections = append(connections, connection)
	}

	return connections
}

func splitInput(input string) *Connection {
	split := strings.Split(input, "->")
	connection := &Connection{
		From: strings.TrimSpace(split[0]),
		To:   strings.TrimSpace(split[1]),
	}
	return connection
}
