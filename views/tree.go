package views

import (
	"strings"
)

type Directory struct {
	Id       string
	Parent   *Directory
	Children []*Directory
}

func (node *Directory) subtree() []*Directory {
	var nodes []*Directory
	nodes = append(nodes, node)
	for _, child := range node.Children {
		nodes = append(nodes, child.subtree()...)
	}
	return nodes
}

func toPaths(nodes []*Directory) []string {
	var paths []string
	for _, node := range nodes {
		paths = append(paths, normalizeDirectoryPath(node.Id))
	}
	return paths
}

func createDirectoryTree(root string, input []string) map[string]*Directory {
	root = normalizeDirectoryPath(root)
	allPaths := fillOutDirectoryTree(root, input)
	lookupTable := make(map[string]*Directory, len(input))

	for _, path := range allPaths {
		lookupTable[path] = &Directory{Id: path, Children: []*Directory{}}
	}
	for dir, node := range lookupTable {
		parent := getParentDirectory(dir)
		parentNode, parentInLookupTable := lookupTable[parent]
		if parentInLookupTable {
			parentNode.Children = append(parentNode.Children, node)
			node.Parent = parentNode
		}
	}
	return lookupTable
}

func normalizeDirectoryPath(root string) string {
	if strings.HasSuffix(root, "/") {
		root = root[:len(root)-1]
	}
	return root
}

// fillOutDirectoryTree makes sure that all paths in the input
// has the proper set of parents.
func fillOutDirectoryTree(root string, input []string) []string {
	allPaths := make(map[string]bool)
	for _, path := range input {
		allPaths[path] = true
		allParents := getAllRequiredParents(root, path)
		for _, parent := range allParents {
			allPaths[parent] = true
		}
	}
	var toReturn []string
	for path, _ := range allPaths {
		toReturn = append(toReturn, path)
	}
	return toReturn
}

func getAllRequiredParents(root, dir string) []string {
	if !strings.HasPrefix(dir, root) || root == dir {
		return []string{}
	}
	var paths []string
	for dir != root || !strings.HasPrefix(dir, root) {
		paths = append(paths, dir)
		dir = getParentDirectory(dir)
	}
	return paths
}
func getParentDirectory(dir string) string {
	if strings.ContainsRune(dir, '/') {
		return dir[:strings.LastIndex(dir, "/")]
	}
	return dir
}
