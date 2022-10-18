package snippets

type ComponentConnection struct {
	From string
	To   string
	File string
}

func GroupConnectionsBy(connections []*ComponentConnection, groupBy func(connection *ComponentConnection) string) map[string][]*ComponentConnection {
	toReturn := make(map[string][]*ComponentConnection)
	for _, connection := range connections {
		group := groupBy(connection)
		toReturn[group] = append(toReturn[group], connection)
	}
	return toReturn
}
