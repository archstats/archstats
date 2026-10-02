package python

import "github.com/archstats/archstats/extensions/treesitter/common"

// complexity reads Python's functions for code health. An else under a loop
// or a try is not an if's, and costs nothing.
var complexity = &common.Complexity{
	Functions:  []string{"function_definition", "lambda"},
	Ifs:        []string{"if_statement"},
	Structures: []string{"for_statement", "while_statement", "except_clause", "conditional_expression", "match_statement"},
	Elses:      []string{"else_clause"},
	ElseIfs:    []string{"elif_clause"},
	Logical:    []string{"boolean_operator"},
	Comments:   []string{"comment"},
	Owners:     []string{"class_definition"},
	Imports:    []string{"import_statement", "import_from_statement"},
}
