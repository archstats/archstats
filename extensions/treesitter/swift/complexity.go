package swift

import (
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// complexity reads Swift's functions for code health. An else is a bare
// token among the if's children; && and || are node kinds of their own; a
// guard is a branch like an if.
var complexity = &common.Complexity{
	Functions: []string{"function_declaration", "init_declaration", "deinit_declaration", "subscript_declaration",
		"computed_property", "lambda_literal"},
	Ifs:         []string{"if_statement"},
	Structures:  []string{"for_statement", "while_statement", "repeat_while_statement", "switch_statement", "guard_statement", "catch_block", "ternary_expression"},
	ElseTokens:  []string{"else"},
	LogicalRuns: []string{"conjunction_expression", "disjunction_expression"},
	Comments:    []string{"comment", "multiline_comment"},
	Owners:      []string{"class_declaration", "protocol_declaration"},
	Imports:     []string{"import_declaration"},
	ParamKinds:  []string{"parameter"},
	NameOf: func(n *tree_sitter.Node, _ []byte) string {
		switch n.Kind() {
		case "init_declaration":
			return "init"
		case "deinit_declaration":
			return "deinit"
		case "subscript_declaration":
			return "subscript"
		}
		return ""
	},
}
