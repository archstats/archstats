package component

import (
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/topo"
)

// Graph is a directed graph that can contain cycles.
// It is a wrapper around gonum's graph.Directed interface.
type Graph struct {
	Name       string
	Components []string

	Connections     []*Connection
	ConnectionsTo   map[string][]*Connection
	ConnectionsFrom map[string][]*Connection

	idMapping  map[string]int64
	components map[int64]*componentNode
	edgesFrom  map[int64][]*componentEdge
	edgesTo    map[int64][]*componentEdge

	shortestCycles              map[string]Cycle
	stronglyConnectedComponents [][]string
}

func (g *Graph) typeAssertion() graph.Directed {
	return g
}

// ShortestCycles returns a map of the shortest cycles in the graph.
// Implements: https://link.springer.com/chapter/10.1007/978-3-642-21952-8_19
func (g *Graph) ShortestCycles() map[string]Cycle {
	if g.shortestCycles == nil {
		g.shortestCycles = shortestCycles(g)
	}
	return g.shortestCycles
}

// StronglyConnectedComponents returns a map of the strongly connected components in the graph.
// Implements: https://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm
func (g *Graph) StronglyConnectedComponents() [][]string {
	if g.stronglyConnectedComponents == nil {
		groups := topo.TarjanSCC(g)

		g.stronglyConnectedComponents = lo.Map(groups, func(group []graph.Node, _ int) []string {
			return lo.Map(group, func(item graph.Node, _ int) string {
				return g.IdToComponent(item.ID())
			})
		})
	}
	return g.stronglyConnectedComponents
}

// DirectPredecessorsOf returns the direct predecessors of a component.
func (g *Graph) DirectPredecessorsOf(component string) []string {
	return lo.Keys(getDirectSidedRelative(component, g.ConnectionsTo, func(connection *Connection) string {
		return connection.From
	}))
}

// DirectSuccessorOf returns the direct successors of a component.
func (g *Graph) DirectSuccessorOf(component string) []string {
	return lo.Keys(getDirectSidedRelative(component, g.ConnectionsFrom, func(connection *Connection) string {
		return connection.To
	}))
}

// AllPredecessorsOf returns all predecessors of a component. Including indirect ones.
func (g *Graph) AllPredecessorsOf(component string) []string {
	return getSidedRelative(component, g.ConnectionsTo, func(connection *Connection) string {
		return connection.From
	})
}

// AllSuccessorsOf returns all successors of a component. Including indirect ones.
func (g *Graph) AllSuccessorsOf(component string) []string {
	return getSidedRelative(component, g.ConnectionsFrom, func(connection *Connection) string {
		return connection.To
	})
}

// CreateGraph creates a new graph with the given name, included components and connections.
// The included components automatically include all components in the connections.
// If you want to include more (orphaned) components, you can add them to the includedComponents slice else pass nil.
func CreateGraph(graphName string, includedComponents []string, connectionsUnpurged []*Connection) *Graph {
	componentIndex := map[string]bool{}

	connections := lo.Filter(connectionsUnpurged, func(connection *Connection, _ int) bool {
		return connection.From != connection.To
	})

	for _, connection := range connections {
		componentIndex[connection.From] = true
		componentIndex[connection.To] = true
	}
	for _, component := range includedComponents {
		componentIndex[component] = true
	}
	amountOfComponents := len(componentIndex)
	idMapping := make(map[string]int64, amountOfComponents)
	allComponents := make(map[int64]*componentNode, amountOfComponents)
	connectionsFromToUnique := getConnectionsWithCount(connections)

	var curId int64
	for componentName := range componentIndex {
		idMapping[componentName] = curId
		allComponents[curId] = &componentNode{
			id:   curId,
			name: componentName,
		}
		curId++
	}

	componentConnectionsByFrom := lo.GroupBy(connections, func(connection *Connection) string {
		return connection.From
	})
	componentConnectionsByTo := lo.GroupBy(connections, func(connection *Connection) string {
		return connection.To
	})

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

	return &Graph{
		Name:            graphName,
		Components:      lo.Keys(componentIndex),
		Connections:     connections,
		ConnectionsFrom: componentConnectionsByFrom,
		ConnectionsTo:   componentConnectionsByTo,
		idMapping:       idMapping,
		components:      allComponents,
		edgesFrom:       edgesFrom,
		edgesTo:         edgesTo,
	}
}

func getConnectionsWithCount(theConnections []*Connection) []*connectionWithCount {
	connections := make([]*connectionWithCount, 0, len(theConnections))
	grouped := lo.GroupBy(theConnections, func(connection *Connection) string {
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

func (g *Graph) IdToComponent(id int64) string {
	return g.components[id].name
}
func (g *Graph) ComponentToId(name string) int64 {
	return g.idMapping[name]
}

func (g *Graph) ComponentToNode(name string) graph.Node {
	return g.components[g.ComponentToId(name)]
}
func (g *Graph) Node(id int64) graph.Node {
	return g.components[id]
}

func (g *Graph) Nodes() graph.Nodes {
	nodes := make([]graph.Node, 0, len(g.components))
	for _, node := range g.components {
		nodes = append(nodes, node)
	}
	return nodeListOf(nodes)
}

func (g *Graph) From(id int64) graph.Nodes {
	nodes := lo.Map(g.edgesFrom[id], func(before *componentEdge, _ int) graph.Node {
		return before.To()
	})
	return nodeListOf(nodes)
}

func (g *Graph) To(id int64) graph.Nodes {
	nodes := lo.Map(g.edgesTo[id], func(before *componentEdge, _ int) graph.Node {
		return before.From()
	})
	return nodeListOf(nodes)
}

func (g *Graph) Edge(xid, yid int64) graph.Edge {
	xEdges := g.edgesFrom[xid]

	for _, edge := range xEdges {
		if edge.to.id == yid {
			return edge
		}
	}
	return nil
}

func (g *Graph) HasEdgeFromTo(xid, yid int64) bool {
	return g.Edge(xid, yid) != nil
}

func (g *Graph) HasEdgeBetween(xid, yid int64) bool {
	return g.Edge(xid, yid) != nil || g.Edge(yid, xid) != nil
}

func (g *Graph) NoConnections() bool {
	return g.Connections == nil || len(g.Connections) == 0
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
