package csharp

import "github.com/archstats/archstats/extensions/treesitter/common"

// complexity reads C#'s functions for code health. A property's get and set
// are functions of their own, named under the property.
var complexity = &common.Complexity{
	Functions: []string{"method_declaration", "constructor_declaration", "destructor_declaration", "operator_declaration",
		"conversion_operator_declaration", "accessor_declaration", "local_function_statement", "lambda_expression", "anonymous_method_expression"},
	Ifs:        []string{"if_statement"},
	Structures: []string{"for_statement", "foreach_statement", "while_statement", "do_statement", "switch_statement", "switch_expression", "catch_clause", "conditional_expression"},
	Logical:    []string{"binary_expression"},
	Comments:   []string{"comment"},
	Owners:     []string{"class_declaration", "struct_declaration", "interface_declaration", "record_declaration", "property_declaration"},
	Imports:    []string{"using_directive"},
}
