package file

// Function is one function, method or named closure as a language pack
// measured it: where it is and how hard it is to follow.
type Function struct {
	// Name is the function's own name, after the types around it:
	// "OrderService.place". Empty for an anonymous function.
	Name string
	// Begin and End are 1-based lines.
	Begin, End int
	// Cognitive is SonarSource's cognitive complexity: one for each break in
	// the linear flow (if, loop, switch, catch, ternary, a run of && or ||),
	// one more for each level it is nested at.
	Cognitive int
	// Nesting is the deepest the function's control flow goes.
	Nesting int
	Params  int
}

// Lines is the function's length, its first and last line included.
func (f *Function) Lines() int { return f.End - f.Begin + 1 }
