package algo

type Vertex[T interface{}] struct {
	Id    string
	Value T
	Edges []*Edge[T]
}

func (v *Vertex[T]) DirectRelatives() []*Vertex[T] {
	panic("implement me")
}
func (v *Vertex[T]) TransitiveRelatives() []*Vertex[T] {
	panic("implement me")

}
func (v *Vertex[T]) DirectSuccessors() []*Vertex[T] {
	panic("implement me")

}
func (v *Vertex[T]) TransitiveSuccessors() []*Vertex[T] {
	panic("implement me")

}
func (v *Vertex[T]) DirectPredecessors() []*Vertex[T] {
	panic("implement me")

}
func (v *Vertex[T]) TransitivePredecessors() []*Vertex[T] {
	panic("implement me")

}

func (v *Vertex[T]) Indegree() int {
	panic("implement me")

}
func (v *Vertex[T]) WeightedIndegree() {
	panic("implement me")

}
func (v *Vertex[T]) Outdegree() int {
	panic("implement me")

}
func (v *Vertex[T]) WeightedOutdegree() int {
	panic("implement me")

}
func (v *Vertex[T]) DegreeSum() int {
	panic("implement me")
}
func (v *Vertex[T]) WeightedDegreeSum() int {
	panic("implement me")

}

func Sources[T interface{}](input []*Vertex[T]) []*Vertex[T] {
	panic("implement me")

}

func Sinks[T interface{}](input []T) []*Vertex[T] {
	panic("implement me")

}

type Edge[T interface{}] struct {
	From  T
	To    T
	Count int
	Type  EdgeType
}
type EdgeType int

const (
	Undirected EdgeType = iota
	Indegree
	Outdegree
)

type UnweightedConnection interface {
	From() string
	To() string
}

type WeightedConnection interface {
	UnweightedConnection
	Weight() int
}

func CreateWeightedDirectedGraph[T interface{}](input []T, edges []WeightedConnection, idFunc func(T) string) map[string]*Vertex[T] {
	lookupTable := make(map[string]*Vertex[T])
	for _, item := range input {
		id := idFunc(item)
		lookupTable[id] = &Vertex[T]{Id: id, Value: item}
	}
	for _, edge := range edges {
		from := edge.From()
		to := edge.To()
		weight := edge.Weight()
		fromNode, ok := lookupTable[from]
		if ok {
			fromNode.Edges = append(fromNode.Edges, &Edge[T]{From: fromNode.Value, To: lookupTable[to].Value, Count: weight, Type: Indegree})
		}
		toNode, ok := lookupTable[to]
		if ok {
			toNode.Edges = append(toNode.Edges, &Edge[T]{From: lookupTable[from].Value, To: toNode.Value, Count: weight, Type: Outdegree})
		}
	}
	return lookupTable
}
