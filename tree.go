package main

type Node struct {
	Id       string
	Parent   *Node
	Children []*Node
}

func (node *Node) Subtree() []*Node {
	var nodes []*Node
	nodes = append(nodes, node)
	for _, child := range node.Children {
		nodes = append(nodes, child.Subtree()...)
	}
	return nodes
}

func ToPaths(nodes []*Node) []string {
	var paths []string
	for _, node := range nodes {
		paths = append(paths, node.Id)
	}
	return paths
}

func createTreesFromList(input []string, parentIdGetter func(string) string) map[string]*Node {
	lookupTable := make(map[string]*Node, len(input))
	for _, s := range input {
		lookupTable[s] = &Node{Id: s}
	}

	for _, node := range lookupTable {
		parent := parentIdGetter(node.Id)
		if parent != "" {
			parentNode := lookupTable[parent]
			parentNode.Children = append(parentNode.Children, node)
			node.Parent = parentNode
		}
	}
	return lookupTable
}
