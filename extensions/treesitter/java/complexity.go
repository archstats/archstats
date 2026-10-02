package java

import "github.com/archstats/archstats/extensions/treesitter/common"

// complexity reads Java's functions for code health. Java writes else-if as
// an if in the alternative field, so it needs no else kinds.
var complexity = &common.Complexity{
	Functions:  []string{"method_declaration", "constructor_declaration", "compact_constructor_declaration", "lambda_expression"},
	Ifs:        []string{"if_statement"},
	Structures: []string{"for_statement", "enhanced_for_statement", "while_statement", "do_statement", "switch_expression", "catch_clause", "ternary_expression"},
	Logical:    []string{"binary_expression"},
	Comments:   []string{"line_comment", "block_comment"},
	Owners:     []string{"class_declaration", "interface_declaration", "enum_declaration", "record_declaration"},
	Imports:    []string{"import_declaration"},
}
