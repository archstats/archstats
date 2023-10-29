package component

import (
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/topo"
	"slices"
	"strings"
)

type Cycle []string

func shortestCycles(theGraph *Graph) map[string]Cycle {
	groups := topo.TarjanSCC(theGraph)

	allEdges := theGraph.Connections

	allShortestCycles := make(map[string]Cycle)

	for _, group := range groups {

		if len(group) == 1 {
			continue
		}

		if len(group) == 2 {
			smallCycle := Cycle{
				theGraph.IdToComponent(group[0].ID()),
				theGraph.IdToComponent(group[1].ID()),
			}
			allShortestCycles[strings.Join(smallCycle, " -> ")] = normalizeCycle(smallCycle)
			continue
		}

		components := lo.Map(group, func(item graph.Node, _ int) string {
			return theGraph.IdToComponent(item.ID())
		})
		componentIdx := lo.Associate(components, func(item string) (string, string) {
			return item, item
		})

		onlyEdgesInGroup := lo.Filter(allEdges, func(edge *Connection, _ int) bool {
			_, okFrom := componentIdx[edge.From]
			_, okTo := componentIdx[edge.To]

			return okTo && okFrom
		})

		cycles := shortestCyclesOfSCC(components, onlyEdgesInGroup)

		for key, cycle := range cycles {
			allShortestCycles[key] = cycle
		}
	}

	return lo.MapEntries(allShortestCycles, func(key string, value Cycle) (string, Cycle) {
		// add first element to the end to make it a cycle
		value = append(value, value[0])
		newKey := strings.Join(value, " -> ")

		return newKey, value
	})
}

func shortestCyclesOfSCC(components []string, connections []*Connection) map[string]Cycle {
	connectionsTo := lo.GroupBy(connections, func(connection *Connection) string {
		return connection.To
	})

	connectionsFrom := lo.GroupBy(connections, func(connection *Connection) string {
		return connection.From
	})
	cycles := make(map[string]Cycle)

	for _, node := range components {

		cyclesForNode := getShortCyclesForNode(node, connectionsTo, connectionsFrom, connections)

		for key, cycle := range cyclesForNode {
			cycles[key] = cycle
		}
	}

	return cycles
}

func getShortCyclesForNode(node string, connectionsTo map[string][]*Connection, connectionsFrom map[string][]*Connection, all []*Connection) map[string]Cycle {

	cycles := make(map[string]Cycle)
	visitedNodes := make(map[string]bool)

	ancestors := getDirectSidedRelative(node, connectionsTo, func(connection *Connection) string {
		return connection.From
	})
	queue := []string{node}

	parentMap := make(map[string]string)

	// BFS stops when every ancestor has been found
	for len(ancestors) > 0 {
		firstElementInQueue := queue[0]
		queue = queue[1:]

		dependencies := connectionsFrom[firstElementInQueue]
		for _, dependencyConnection := range dependencies {

			dependency := dependencyConnection.To
			currentNode := dependencyConnection.From
			_, isVisited := visitedNodes[dependency]
			visitedNodes[firstElementInQueue] = true

			inQueue := lo.Contains(queue, dependency)

			if !isVisited && !inQueue {
				parentMap[dependency] = currentNode
				queue = append(queue, dependency)
			}
			_, isDirectAncestorOfOriginalNode := ancestors[dependency]
			// Cycle found
			if isDirectAncestorOfOriginalNode {
				// build cycle by backtracking parents
				var cycleToAdd []string
				i := dependency
				for i != "" {
					cycleToAdd = append(cycleToAdd, i)
					i = parentMap[i]
				}
				// reverse, because backtracking.... (I spent 4 hours wondering if I just forgot how to do BFS)
				slices.Reverse(cycleToAdd)

				cycleToAdd = normalizeCycle(cycleToAdd)

				cycleKey := strings.Join(cycleToAdd, " -> ")
				cycles[cycleKey] = cycleToAdd

				//remove from ancestors
				delete(ancestors, dependency)
			}
		}
	}

	return cycles
}

// Rotate the list of strings so that the first string is the smallest.
func normalizeCycle(nodes Cycle) Cycle {
	// Find the index of the smallest string.
	var minIndex int
	for i, node := range nodes {
		if node < nodes[minIndex] {
			minIndex = i
		}
	}
	// Rotate the list so that the smallest string is first.
	return append(nodes[minIndex:], nodes[:minIndex]...)
}

func splitIntoPairs(strs []string) [][]string {
	pairs := make([][]string, len(strs)-1)
	for i := 0; i < len(strs); i++ {
		if i == len(strs)-1 {
			continue
		} else {
			pair := []string{strs[i], strs[i+1]}
			pairs[i] = pair
		}
	}
	return pairs
}
