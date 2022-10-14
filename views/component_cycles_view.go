package views

import (
	"github.com/RyanSusana/archstats/snippets"
	"sort"
	"strings"
)

func ComponentCyclesView(results *snippets.Results) *View {
	connections := getConnectionsWithCount(results)
	connectionsToIndex := results.ConnectionsTo

	cycles := allCycles(connections, connectionsToIndex)

	var rows []*Row
	for i, theCycle := range cycles {

		for _, component := range theCycle {
			successor := theCycle[i+1%len(theCycle)]
			predecessor := theCycle[i-1%len(theCycle)]
			rows = append(rows, &Row{
				Data: map[string]interface{}{
					"cycle_nr":    i,
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

func allCycles(connections []*connectionWithCount, connectionsToIndex map[string][]*snippets.ComponentConnection) []cycle {
	visited := make(map[string]cycle)

	for _, connection := range connections {
		theCycles := allCyclesForComponent(connection.from, connectionsToIndex)
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

func allCyclesForComponent(component string, connectionsToIndex map[string][]*snippets.ComponentConnection) []cycle {
	cycles := make([]cycle, 0)
	visited := make(map[string]bool)

	stack := make([]string, 0)
	directSuccessors := connectionsToIndex[component]

	for _, successor := range directSuccessors {
		findCycles(successor.To, component, connectionsToIndex, visited, stack, cycles)
	}

	return cycles
}

func findCycles(currentSuccessor string, currentlyTrackedComponent string, index map[string][]*snippets.ComponentConnection, alreadyVisited map[string]bool, stack []string, cycles []cycle) {
	if alreadyVisited[currentSuccessor] {
		return
	}

	alreadyVisited[currentSuccessor] = true
	stack = append(stack, currentSuccessor)

	//if the current currentSuccessor is the same as the tracked currentlyTrackedComponent, we can return the stack as the cycle.
	//otherwise, we need to find the cycles of the currentSuccessor
	if currentSuccessor == currentlyTrackedComponent {
		cycles = append(cycles, stack)
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
