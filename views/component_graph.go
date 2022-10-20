package views

import (
	"github.com/RyanSusana/archstats/analysis"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
)

func createGraph(results *analysis.Results) *componentGraph {
	amountOfComponents := len(results.SnippetsByComponent)
	idMapping := make(map[string]int64, amountOfComponents)
	allComponents := make(map[int64]*componentNode, amountOfComponents)
	connectionsFromToUnique := getConnectionsWithCount(results)

	var curId int64
	for componentName := range results.SnippetsByComponent {
		idMapping[componentName] = curId
		allComponents[curId] = &componentNode{
			id:   curId,
			name: componentName,
		}
		curId++
	}

	edgesFrom := make(map[int64][]*componentEdge, len(connectionsFromToUnique))
	edgesTo := make(map[int64][]*componentEdge, len(connectionsFromToUnique))
	for _, connection := range connectionsFromToUnique {

		fromId := idMapping[connection.from]
		toId := idMapping[connection.to]
		edgesFrom[fromId] = append(edgesFrom[fromId], &componentEdge{
			from: allComponents[fromId],
			to:   allComponents[toId],
		})

		edgesTo[toId] = append(edgesTo[toId], &componentEdge{
			from: allComponents[fromId],
			to:   allComponents[toId],
		})
	}

	return &componentGraph{
		idMapping:  idMapping,
		components: allComponents,
		edgesFrom:  edgesFrom,
		edgesTo:    edgesTo,
	}
}

func getConnectionsWithCount(results *analysis.Results) []*connectionWithCount {
	connections := make([]*connectionWithCount, 0, len(results.Connections))
	grouped := lo.GroupBy(results.Connections, func(connection *analysis.ComponentConnection) string {
		return connection.From + " -> " + connection.To
	})

	for connectionName, groupedConnections := range grouped {
		connections = append(connections, &connectionWithCount{
			name:  connectionName,
			count: len(groupedConnections),
			from:  groupedConnections[0].From,
			to:    groupedConnections[0].To,
		})
	}
	return connections
}

type connectionWithCount struct {
	name  string
	from  string
	to    string
	count int
}

type componentNode struct {
	id   int64
	name string
}

func (c *componentNode) String() string {
	return c.name
}

func (c *componentNode) ID() int64 {
	return c.id
}

type componentEdge struct {
	from *componentNode
	to   *componentNode
}

func (c *componentEdge) String() string {
	return c.from.String() + " -> " + c.to.String()
}

func (c *componentEdge) From() graph.Node {
	return c.from
}
func (c *componentEdge) To() graph.Node {
	return c.to
}

func (c *componentEdge) ReversedEdge() graph.Edge {
	return &componentEdge{
		from: c.to,
		to:   c.from,
	}
}

type componentGraph struct {
	idMapping  map[string]int64
	components map[int64]*componentNode
	edgesFrom  map[int64][]*componentEdge
	edgesTo    map[int64][]*componentEdge
}

func (c *componentGraph) ComponentId(name string) int64 {
	return c.idMapping[name]
}

func (c *componentGraph) Component(name string) graph.Node {
	return c.components[c.ComponentId(name)]
}
func (c *componentGraph) Node(id int64) graph.Node {
	return c.components[id]
}

func (c *componentGraph) Nodes() graph.Nodes {
	nodes := make([]graph.Node, 0, len(c.components))
	for _, node := range c.components {
		nodes = append(nodes, node)
	}
	return nodeListOf(nodes)
}

func (c *componentGraph) From(id int64) graph.Nodes {
	nodes := lo.Map(c.edgesFrom[id], func(before *componentEdge, _ int) graph.Node {
		return before.To()
	})
	return nodeListOf(nodes)
}

func (c *componentGraph) To(id int64) graph.Nodes {
	nodes := lo.Map(c.edgesTo[id], func(before *componentEdge, _ int) graph.Node {
		return before.From()
	})
	return nodeListOf(nodes)
}

func (c *componentGraph) Edge(xid, yid int64) graph.Edge {
	xEdges := c.edgesFrom[xid]

	for _, edge := range xEdges {
		if edge.to.id == yid {
			return edge
		}
	}
	return nil
}

func (c *componentGraph) HasEdgeFromTo(xid, yid int64) bool {
	return c.Edge(xid, yid) != nil
}

func (c *componentGraph) HasEdgeBetween(xid, yid int64) bool {
	return c.Edge(xid, yid) != nil || c.Edge(yid, xid) != nil
}

func nodeListOf(nodes []graph.Node) graph.Nodes {
	return &nodeList{
		nodes:   nodes,
		curNode: -1,
	}
}

type nodeList struct {
	nodes   []graph.Node
	curNode int
}

func (n *nodeList) Next() bool {
	n.curNode++
	return n.curNode < len(n.nodes)
}

func (n *nodeList) Len() int {
	return len(n.nodes)
}

func (n *nodeList) Reset() {
	n.curNode = -1
}

func (n *nodeList) Node() graph.Node {
	return n.nodes[n.curNode]
}
