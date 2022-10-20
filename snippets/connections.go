package snippets

// ComponentConnection is a connection between two components.
type ComponentConnection struct {
	From string
	To   string
	//The file in which the connection is made. The from side.
	File string
}
