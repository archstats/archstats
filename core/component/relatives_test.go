package component

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetAncestors_Simple(t *testing.T) {

	input := []string{
		"a -> b",
		"b -> c",
		"c -> d",
		"d -> e",
		"e -> f",
		"f -> g",
		"g -> h",
	}

	graph := connectionStringsToGraph(input)

	ancestors := graph.AllPredecessorsOf("h")
	assert.Len(t, ancestors, 7)

}

func TestGetAncestors_DirectedCyclicalGraph(t *testing.T) {
	input := []string{
		"a -> b",

		"b -> c",
		"b -> d",

		"c -> e",
		"c -> f",
		"c -> g",
		"c -> h",

		"d -> i",
		"d -> j",
		"d -> k",
		"d -> l",

		"l -> b",
		"l -> c",
		"l -> d",
		"l -> e",
		"l -> f",
		"l -> g",
		"l -> h",

		"e -> m",
		"e -> n",
		"e -> o",
		"e -> p",
	}
	graph := connectionStringsToGraph(input)
	tests := map[string][]string{
		"a": {},
		"b": {"a", "l", "d"},
		"c": {"a", "l", "d", "b"},
		"d": {"a", "l", "b"},
		"e": {"a", "l", "d", "b", "c"},
		"f": {"a", "l", "d", "b", "c"},
		"g": {"a", "l", "d", "b", "c"},
		"h": {"a", "l", "d", "b", "c"},
		"i": {"a", "l", "d", "b"},
		"j": {"a", "l", "d", "b"},
		"k": {"a", "l", "d", "b"},
		"l": {"a", "d", "b"},
		"m": {"a", "l", "d", "b", "c", "e"},
		"n": {"a", "l", "d", "b", "c", "e"},
		"o": {"a", "l", "d", "b", "c", "e"},
		"p": {"a", "l", "d", "b", "c", "e"},
	}
	for cmpnt, expectedAncestors := range tests {
		t.Run(fmt.Sprintf("test DCG ancestors for '%s'", cmpnt), func(t *testing.T) {
			actualAncestors := graph.AllPredecessorsOf(cmpnt)
			assert.Len(t, actualAncestors, len(expectedAncestors))
			assert.ElementsMatch(t, actualAncestors, expectedAncestors)
		})
	}
}

func TestGetAncestors_DirectedAcyclicalGraph(t *testing.T) {
	input := []string{
		"a -> b",

		"b -> c",
		"b -> d",

		"c -> e",
		"c -> f",
		"c -> g",
		"c -> h",

		"d -> i",
		"d -> j",
		"d -> k",
		"d -> l",
	}
	graph := connectionStringsToGraph(input)
	tests := map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a", "b"},
		"d": {"a", "b"},
		"e": {"a", "b", "c"},
		"f": {"a", "b", "c"},
		"g": {"a", "b", "c"},
		"h": {"a", "b", "c"},
		"i": {"a", "b", "d"},
		"j": {"a", "b", "d"},
		"k": {"a", "b", "d"},
		"l": {"a", "b", "d"},
	}
	for cmpnt, expectedAncestors := range tests {
		t.Run(fmt.Sprintf("test DAG ancestors for '%s'", cmpnt), func(t *testing.T) {
			actualAncestors := graph.AllPredecessorsOf(cmpnt)
			assert.Len(t, expectedAncestors, len(actualAncestors))
			assert.ElementsMatch(t, expectedAncestors, actualAncestors)
		})
	}
}

func TestGetAncestors_LinkedList(t *testing.T) {
	input := []string{
		"a -> b",
		"b -> c",
		"c -> d",
		"d -> e",
		"e -> f",
		"f -> g",
		"g -> h",
	}
	graph := connectionStringsToGraph(input)
	tests := map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a", "b"},
		"d": {"a", "b", "c"},
		"e": {"a", "b", "c", "d"},
		"f": {"a", "b", "c", "d", "e"},
		"g": {"a", "b", "c", "d", "e", "f"},
		"h": {"a", "b", "c", "d", "e", "f", "g"},
	}
	for cmpnt, expectedAncestors := range tests {
		t.Run(fmt.Sprintf("test linked list ancestors for '%s'", cmpnt), func(t *testing.T) {
			actualAncestors := graph.AllPredecessorsOf(cmpnt)
			assert.Len(t, actualAncestors, len(expectedAncestors))
			assert.ElementsMatch(t, actualAncestors, expectedAncestors)
		})
	}
}

func connectionStringsToGraph(inputs []string) *Graph {
	connections := connectionStringsToConnections(inputs)
	return CreateGraph(nil, connections)
}
