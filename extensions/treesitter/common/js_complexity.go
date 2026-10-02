package common

// JSComplexity reads JavaScript's and TypeScript's functions for code
// health. An arrow function assigned to a name takes that name; one passed
// as an argument nests inside the function that passes it.
var JSComplexity = &Complexity{
	Functions: []string{"function_declaration", "generator_function_declaration", "method_definition", "arrow_function",
		"function_expression", "function", "generator_function"},
	Ifs:         []string{"if_statement"},
	Structures:  []string{"for_statement", "for_in_statement", "while_statement", "do_statement", "switch_statement", "catch_clause", "ternary_expression"},
	Elses:       []string{"else_clause"},
	Logical:     []string{"binary_expression"},
	Comments:    []string{"comment"},
	Owners:      []string{"class_declaration", "class", "abstract_class_declaration"},
	Imports:     []string{"import_statement"},
	ImportCalls: []string{"require"},
}
