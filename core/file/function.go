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
	// Signature is the declaration as written up to its body, on one line:
	// `public Order place(Cart cart, Customer customer)`. Annotations and
	// decorators are left off; they are markers, not the signature.
	Signature string
	// Increments are what the cognitive complexity is made of, in the order
	// the function reads.
	Increments []Increment
}

// Increment is one step of a function's cognitive complexity: the construct
// on a line, and what it cost there.
type Increment struct {
	Line int
	// Points is 1 for a branch, plus the nesting it sits at for an if, loop,
	// switch, catch or ternary.
	Points int
	// Construct is the keyword the code spells it with: if, else if, for,
	// while, catch, when, guard, ?, &&, ||.
	Construct string
	Nesting   int
}

// Lines is the function's length, its first and last line included.
func (f *Function) Lines() int { return f.End - f.Begin + 1 }
