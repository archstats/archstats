package views

import (
	"github.com/RyanSusana/archstats/snippets"
	"sort"
	"strings"
)

func ComponentCyclesView(results *snippets.Results) *View {
	connections := getConnectionsWithCount(results)
	connectionsIndex := results.ConnectionsFrom

	cycles := allCycles(connections, connectionsIndex)

	sort.Slice(cycles, func(c1, c2 int) bool {
		return len(cycles[c1]) < len(cycles[c2])
	})

	var rows []*Row
	for cycleNr, theCycle := range cycles {
		for componentIndex, component := range theCycle {
			successor := theCycle[wrapIndex(componentIndex+1, len(theCycle))]
			predecessor := theCycle[wrapIndex(componentIndex-1, len(theCycle))]
			rows = append(rows, &Row{
				Data: map[string]interface{}{
					"cycle_nr":    cycleNr,
					"cycle_size":  len(theCycle),
					"component":   component,
					"successor":   successor,
					"predecessor": predecessor,
				},
			})
		}

	}
	return &View{
		OrderedColumns: []string{"cycle_nr", "cycle_size", "component", "successor", "predecessor"},
		Rows:           rows,
	}
}

func allCycles(connections []*connectionWithCount, connectionsIndex map[string][]*snippets.ComponentConnection) []cycle {
	visited := make(map[string]cycle)

	for _, connection := range connections {
		theCycles := allCyclesForComponent(connection.from, connectionsIndex)
		for _, theCycle := range theCycles {
			visited[theCycle.getKey()] = theCycle
		}
	}
	cycles := make([]cycle, 0, len(visited))
	for _, theCycle := range visited {
		cycles = append(cycles, theCycle)
	}
	return cycles
}

func allCyclesForComponent(component string, connectionsIndex map[string][]*snippets.ComponentConnection) []cycle {
	cycles := make([]cycle, 0)
	visited := make(map[string]bool)

	stack := make([]string, 0)
	directSuccessors := connectionsIndex[component]

	for _, successor := range directSuccessors {
		findCycles(successor.To, component, connectionsIndex, visited, stack, &cycles)
	}

	return cycles
}

func findCycles(currentSuccessor string, currentlyTrackedComponent string, index map[string][]*snippets.ComponentConnection, alreadyVisited map[string]bool, stack []string, cycles *[]cycle) {
	if alreadyVisited[currentSuccessor] {
		return
	}

	alreadyVisited[currentSuccessor] = true
	stack = append(stack, currentSuccessor)

	//if the current currentSuccessor is the same as the tracked currentlyTrackedComponent, we can return the stack as the cycle.
	//otherwise, we need to find the cycles of the currentSuccessor
	if currentSuccessor == currentlyTrackedComponent {
		*cycles = append(*cycles, stack)
	} else {
		directSuccessors := index[currentSuccessor]
		for _, nextSuccessor := range directSuccessors {
			findCycles(nextSuccessor.To, currentlyTrackedComponent, index, alreadyVisited, stack, cycles)
		}
	}

	//pop
	stack = stack[:len(stack)-1]
	alreadyVisited[currentSuccessor] = false
}

type cycle []string

// the cycle must retain the order of the components
func (c cycle) getKey() string {
	sorted := make([]string, len(c))
	copy(sorted, c)
	sort.Strings(sorted)
	return strings.Join(sorted, " -> ")
}

func wrapIndex(i, max int) int {
	i = i % max
	if i < 0 {
		i += max
	}
	return i
}
