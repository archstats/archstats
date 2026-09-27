package components

import (
	"sort"

	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/component"
	"github.com/samber/lo"
)

// ConnectionsIndirectView lists every component that reaches another, how
// many hops the shortest route takes, and the first hop of that route.
//
// It used to store the whole route as text ("a -> b -> c -> d") on every
// pair. With one row per reachable pair that was most of a large snapshot
// (166 MB of 263 on fineract) for a column one screen reads for one pair at a
// time. The route is still there: follow next_hop from `from` towards `to`,
// reading the row for (next_hop, to) each step. Every step lands one hop
// closer, so the walk is a shortest route and ends at `to`.
//
// Among tied first hops the alphabetically first is kept, so a walk spells
// the alphabetically first shortest route, as the stored text did.
func ConnectionsIndirectView(results *core.Results) *core.View {
	theGraph := results.ComponentGraph

	successors := map[string][]string{}
	predecessors := map[string][]string{}
	for from, connections := range theGraph.ConnectionsFrom {
		successors[from] = lo.Uniq(lo.Map(connections, func(c *component.Connection, _ int) string { return c.To }))
		sort.Strings(successors[from])
	}
	for to, connections := range theGraph.ConnectionsTo {
		predecessors[to] = lo.Uniq(lo.Map(connections, func(c *component.Connection, _ int) string { return c.From }))
	}

	components := lo.Keys(results.SnippetsByComponent)
	sort.Strings(components)
	listed := lo.SliceToMap(components, func(c string) (string, bool) { return c, true })

	var rows []*core.Row
	for _, to := range components {
		// Hops from every component to `to`, walking the edges backwards.
		hops := map[string]int{to: 0}
		queue := []string{to}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, before := range predecessors[current] {
				if _, seen := hops[before]; !seen {
					hops[before] = hops[current] + 1
					queue = append(queue, before)
				}
			}
		}

		for from, distance := range hops {
			if from == to || !listed[from] {
				continue
			}
			next := ""
			for _, candidate := range successors[from] {
				if d, reaches := hops[candidate]; reaches && d == distance-1 {
					next = candidate
					break
				}
			}
			rows = append(rows, &core.Row{
				Data: map[string]interface{}{
					"from":                 from,
					"to":                   to,
					"shortest_path_length": distance,
					"next_hop":             next,
				},
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i].Data, rows[j].Data
		if a["from"] != b["from"] {
			return a["from"].(string) < b["from"].(string)
		}
		return a["to"].(string) < b["to"].(string)
	})

	return &core.View{
		Columns: []*core.Column{core.StringColumn("from"), core.StringColumn("to"), core.IntColumn("shortest_path_length"), core.StringColumn("next_hop")},
		Rows:    rows,
	}
}
