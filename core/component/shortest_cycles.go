package component

import (
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/topo"
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

		cyclesForNode := getShortCyclesForNode(node, connectionsTo, connectionsFrom)

		for key, cycle := range cyclesForNode {
			cycles[key] = cycle
		}
	}

	return cycles
}

func getShortCyclesForNode(node string, connectionsTo map[string][]*Connection, connectionsFrom map[string][]*Connection) map[string]Cycle {
	cycles := make(map[string]Cycle)
	visitedNodes := make(map[string]bool)
	ancestors := getDirectSidedRelative(node, connectionsTo, func(connection *Connection) string {
		return connection.From
	})
	var currentPath []string
	queue := []string{node}

	// BFS stops when every ancestor has been found
	for len(ancestors) > 0 {
		firstElementInQueue := queue[0]
		queue = queue[1:]

		dependencies := connectionsFrom[firstElementInQueue]
		for _, dependencyConnection := range dependencies {

			dependency := dependencyConnection.To
			currentNode := dependencyConnection.From
			_, isVisited := visitedNodes[dependency]
			inQueue := lo.Contains(queue, dependency)

			if !isVisited && !inQueue {
				if len(currentPath) == 0 || currentPath[len(currentPath)-1] != currentNode {
					currentPath = append(currentPath, currentNode)
				}
				queue = append(queue, dependency)
			}
			_, isDirectAncestorOfOriginalNode := ancestors[dependency]
			// Cycle found
			if isDirectAncestorOfOriginalNode {
				var cycleToAdd Cycle
				cycleToAdd = append(cycleToAdd, currentPath...)
				cycleToAdd = append(cycleToAdd, dependency)

				cycleToAdd = normalizeCycle(cycleToAdd)

				cycleKey := strings.Join(cycleToAdd, " -> ")
				cycles[cycleKey] = cycleToAdd

				//remove from ancestors
				delete(ancestors, dependency)
			}
		}

		visitedNodes[firstElementInQueue] = true
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
