package views

import (
	"fmt"
	"github.com/RyanSusana/archstats/snippets"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestNoCycles(t *testing.T) {
	input := constructInput([]string{
		"A -> B",
		"A -> C",
		"B -> D",
	})

	results := ComponentConnectionsView(input).Rows

	assert.Len(t, results, 0)
}

func TestOneSimpleCycle(t *testing.T) {
	input := constructInput([]string{
		"A -> B",
		"B -> A",
	})

	results := ComponentConnectionsView(input).Rows
	assert.Len(t, results, 2)
}

func TestOneComplexCycle(t *testing.T) {
	input := constructInput([]string{
		"A -> B",
		"B -> C",
		"C -> D",
		"D -> A",
	})

	results := ComponentConnectionsView(input).Rows

	assert.Len(t, results, 4)
}

func totalCycles(rows []*Row) int {
	return len(groupBy(rows, func(row *Row) string {
		return fmt.Sprintf("%v", row.Data["cycle_nr"])
	}))
}

func constructInput(inputs []string) *snippets.Results {

	connections := make([]*snippets.ComponentConnection, 0, len(inputs))

	for _, input := range inputs {

		split := strings.Split(input, "->")
		connection := &snippets.ComponentConnection{
			From: strings.TrimSpace(split[0]),
			To:   strings.TrimSpace(split[1]),
		}
		connections = append(connections, connection)
	}

	return &snippets.Results{
		Connections: connections,
		ConnectionsFrom: groupBy(connections, func(connection *snippets.ComponentConnection) string {
			return connection.From
		}),
		ConnectionsTo: groupBy(connections, func(connection *snippets.ComponentConnection) string {
			return connection.To
		}),
	}
}
