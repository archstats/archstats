package golang

import "github.com/archstats/archstats/extensions/treesitter/common"

// complexity reads Go's functions for code health. A method is named under
// its receiver's type.
var complexity = &common.Complexity{
	Functions:  []string{"function_declaration", "method_declaration", "func_literal"},
	Ifs:        []string{"if_statement"},
	Structures: []string{"for_statement", "expression_switch_statement", "type_switch_statement", "select_statement"},
	Logical:    []string{"binary_expression"},
	Comments:   []string{"comment"},
	Imports:    []string{"import_spec"},
}
