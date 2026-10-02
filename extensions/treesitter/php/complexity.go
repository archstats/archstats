package php

import "github.com/archstats/archstats/extensions/treesitter/common"

// complexity reads PHP's functions for code health. `elseif` is its own
// clause; `else if` is an if inside an else.
var complexity = &common.Complexity{
	Functions:  []string{"function_definition", "method_declaration", "anonymous_function", "anonymous_function_creation_expression", "arrow_function"},
	Ifs:        []string{"if_statement"},
	Structures: []string{"for_statement", "foreach_statement", "while_statement", "do_statement", "switch_statement", "match_expression", "catch_clause", "conditional_expression"},
	Elses:      []string{"else_clause"},
	ElseIfs:    []string{"else_if_clause"},
	Logical:    []string{"binary_expression"},
	Comments:   []string{"comment"},
	Owners:     []string{"class_declaration", "interface_declaration", "trait_declaration", "enum_declaration"},
	Imports:    []string{"namespace_use_clause"},
}
