package kotlin

import "github.com/archstats/archstats/extensions/treesitter/common"

// complexity reads Kotlin's functions for code health. The grammar names no
// fields, so a declaration's first identifier is its name; an else-if is a
// body holding nothing but an if; && and || are node kinds of their own.
var complexity = &common.Complexity{
	Functions: []string{"function_declaration", "secondary_constructor", "anonymous_initializer", "getter", "setter",
		"anonymous_function", "lambda_literal"},
	Ifs:            []string{"if_expression"},
	Structures:     []string{"for_statement", "while_statement", "do_while_statement", "when_expression", "catch_block"},
	ElseIfWrappers: []string{"control_structure_body"},
	LogicalRuns:    []string{"conjunction_expression", "disjunction_expression"},
	Comments:       []string{"line_comment", "multiline_comment"},
	Owners:         []string{"class_declaration", "object_declaration", "companion_object"},
	Imports:        []string{"import_header"},
	NameKinds:      []string{"simple_identifier", "type_identifier"},
	ParamKinds:     []string{"parameter"},
}
