package java

import (
	"github.com/archstats/archstats/core"
	"github.com/samber/lo"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"strings"
)

type classNode struct {
	id   int64
	name string
}

func (n *classNode) ID() int64 { return n.id }

type classEdge struct {
	from *classNode
	to   *classNode
}

func (e *classEdge) From() graph.Node { return e.from }
func (e *classEdge) To() graph.Node   { return e.to }
func (e *classEdge) ReversedEdge() graph.Edge {
	return &classEdge{from: e.to, to: e.from}
}

type ClassGraph struct {
	nodes      map[int64]*classNode
	nodeMap    map[string]*classNode
	adjacency  map[int64]map[int64]bool
	rAdjacency map[int64]map[int64]bool
}

func NewClassGraph(classes []string, connections []*classConnection) *ClassGraph {
	g := &ClassGraph{
		nodes:      make(map[int64]*classNode),
		nodeMap:    make(map[string]*classNode),
		adjacency:  make(map[int64]map[int64]bool),
		rAdjacency: make(map[int64]map[int64]bool),
	}

	var id int64
	for _, cls := range classes {
		node := &classNode{id: id, name: cls}
		g.nodes[id] = node
		g.nodeMap[cls] = node
		g.adjacency[id] = make(map[int64]bool)
		g.rAdjacency[id] = make(map[int64]bool)
		id++
	}

	for _, conn := range connections {
		fromNode, fromExists := g.nodeMap[conn.from]
		toNode, toExists := g.nodeMap[conn.to]
		if fromExists && toExists {
			g.adjacency[fromNode.id][toNode.id] = true
			g.rAdjacency[toNode.id][fromNode.id] = true
		}
	}

	return g
}

func (g *ClassGraph) Node(id int64) graph.Node {
	return g.nodes[id]
}

func (g *ClassGraph) Nodes() graph.Nodes {
	var list []graph.Node
	for _, n := range g.nodes {
		list = append(list, n)
	}
	return nodeListOf(list)
}

func (g *ClassGraph) From(id int64) graph.Nodes {
	var list []graph.Node
	for targetID := range g.adjacency[id] {
		list = append(list, g.nodes[targetID])
	}
	return nodeListOf(list)
}

func (g *ClassGraph) To(id int64) graph.Nodes {
	var list []graph.Node
	for sourceID := range g.rAdjacency[id] {
		list = append(list, g.nodes[sourceID])
	}
	return nodeListOf(list)
}

func (g *ClassGraph) HasEdgeBetween(xid, yid int64) bool {
	return g.adjacency[xid][yid] || g.adjacency[yid][xid]
}

func (g *ClassGraph) Edge(xid, yid int64) graph.Edge {
	if g.adjacency[xid][yid] {
		return &classEdge{from: g.nodes[xid], to: g.nodes[yid]}
	}
	return nil
}

func (g *ClassGraph) HasEdgeFromTo(xid, yid int64) bool {
	return g.adjacency[xid][yid]
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

func nodeListOf(nodes []graph.Node) graph.Nodes {
	return &nodeList{
		nodes:   nodes,
		curNode: -1,
	}
}

type classConnection struct {
	from string
	to   string
}

func ClassConnectionsDirectView(results *core.Results) *core.View {
	fileToClass := make(map[string]string)
	classToFile := make(map[string]string)

	for f, statsList := range results.StatRecordsByFile {
		merged := results.Calculate(statsList)
		if val, exists := (*merged)["java_full_class"]; exists {
			if classStr, ok := val.(string); ok && classStr != "" {
				fileToClass[f] = classStr
				classToFile[classStr] = f
			}
		}
	}

	var rows []*core.Row
	for f, classFrom := range fileToClass {
		snippets := results.SnippetsByFile[f]
		for _, snippet := range snippets {
			if snippet.Type == "java__import__declaration" {
				importedClass := snippet.Value
				if targetFile, exists := classToFile[importedClass]; exists {
					classTo := fileToClass[targetFile]
					if classFrom == classTo {
						continue
					}
					rows = append(rows, &core.Row{
						Data: map[string]interface{}{
							"from":            classFrom,
							"to":              classTo,
							"file":            f,
							"reference_count": 1,
						},
					})
				}
			}
		}
	}

	return &core.View{
		Name: "java_class_connections_direct",
		Columns: []*core.Column{
			core.StringColumn("from"),
			core.StringColumn("to"),
			core.StringColumn("file"),
			core.IntColumn("reference_count"),
		},
		Rows: rows,
	}
}

func ClassConnectionsIndirectView(results *core.Results) *core.View {
	fileToClass := make(map[string]string)
	classToFile := make(map[string]string)
	var classes []string

	for f, statsList := range results.StatRecordsByFile {
		merged := results.Calculate(statsList)
		if val, exists := (*merged)["java_full_class"]; exists {
			if classStr, ok := val.(string); ok && classStr != "" {
				fileToClass[f] = classStr
				classToFile[classStr] = f
				classes = append(classes, classStr)
			}
		}
	}

	var directConns []*classConnection
	seenConns := make(map[string]bool)
	for f, classFrom := range fileToClass {
		snippets := results.SnippetsByFile[f]
		for _, snippet := range snippets {
			if snippet.Type == "java__import__declaration" {
				importedClass := snippet.Value
				if targetFile, exists := classToFile[importedClass]; exists {
					classTo := fileToClass[targetFile]
					if classFrom == classTo {
						continue
					}
					key := classFrom + " -> " + classTo
					if !seenConns[key] {
						seenConns[key] = true
						directConns = append(directConns, &classConnection{from: classFrom, to: classTo})
					}
				}
			}
		}
	}

	g := NewClassGraph(classes, directConns)
	allShortest := path.DijkstraAllPaths(g)

	var rows []*core.Row
	for _, from := range classes {
		for _, to := range classes {
			if from == to {
				continue
			}

			fromNode := g.nodeMap[from]
			toNode := g.nodeMap[to]
			if fromNode == nil || toNode == nil {
				continue
			}

			shortestPaths, _ := allShortest.AllBetween(fromNode.id, toNode.id)

			for _, shortest := range shortestPaths {
				if len(shortest) >= 2 {
					rows = append(rows, &core.Row{
						Data: map[string]interface{}{
							"from":                 from,
							"to":                   to,
							"shortest_path_length": len(shortest),
							"shortest_path": strings.Join(lo.Map(
								shortest,
								func(node graph.Node, _ int) string {
									return g.nodes[node.ID()].name
								},
							), " -> "),
						},
					})
				}
			}
		}
	}

	return &core.View{
		Name: "java_class_connections_indirect",
		Columns: []*core.Column{
			core.StringColumn("from"),
			core.StringColumn("to"),
			core.IntColumn("shortest_path_length"),
			core.StringColumn("shortest_path"),
		},
		Rows: rows,
	}
}
