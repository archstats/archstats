package algo

type TreeNode[T interface{}] struct {
	Value    T
	Parent   *TreeNode[T]
	Children []*TreeNode[T]
}

func TreeFromSlice[T interface{}](input map[string]T, idFunc func(T) string, parentIdFunc func(T) string) map[string]*TreeNode[T] {
	lookupTable := make(map[string]*TreeNode[T])
	for _, item := range input {
		id := idFunc(item)
		lookupTable[id] = &TreeNode[T]{Value: item}
	}
	for _, item := range input {
		id := idFunc(item)
		parentId := parentIdFunc(item)
		if parentId != "" {
			parentNode, ok := lookupTable[parentId]
			if ok {
				parentNode.Children = append(parentNode.Children, lookupTable[id])
				lookupTable[id].Parent = parentNode
			}
		}
	}
	return lookupTable
}
