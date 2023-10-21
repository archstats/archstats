package component

func getDirectSidedRelative(
	node string,
	connectionsTo map[string][]*Connection,
	otherSide func(connection *Connection) string,
) map[string]bool {
	allConnectionsTo := connectionsTo[node]

	ancestors := make(map[string]bool, len(allConnectionsTo))

	for _, connection := range allConnectionsTo {
		ancestors[otherSide(connection)] = true
	}
	return ancestors
}

func getSidedRelative(
	node string,
	edgesIndexByTo map[string][]*Connection,
	otherSide func(connection *Connection) string,
) []string {

	// Create a set to store the unique ancestors.
	ancestors := map[string]struct{}{}

	// Perform a DFS starting from the given node.
	stack := []string{node}
	for len(stack) > 0 {
		currentNode := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Add the current node to the set of ancestors.
		ancestors[currentNode] = struct{}{}

		// Visit all of the current node's ancestors.
		for _, connection := range edgesIndexByTo[currentNode] {
			from := otherSide(connection)
			if _, ok := ancestors[from]; ok {
				continue
			}
			stack = append(stack, from)
		}
	}

	// Remove the given node from the set of ancestors.
	delete(ancestors, node)

	// Convert the set of ancestors to a list and return it.
	var ancestorsList []string
	for ancestor := range ancestors {
		ancestorsList = append(ancestorsList, ancestor)
	}
	return ancestorsList
}
