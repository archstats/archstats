package dart

import (
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// complexity reads Dart's functions for code health. A Dart function is a
// signature followed by a body, side by side: the body is the function and
// the signature before it gives the name and the parameters.
var complexity = &common.Complexity{
	Functions:      []string{"function_expression"},
	FunctionBodies: []string{"function_body"},
	Ifs:            []string{"if_statement"},
	Structures: []string{"for_statement", "while_statement", "do_statement", "switch_statement", "switch_expression",
		"catch_clause", "conditional_expression", "for_element", "if_element"},
	LogicalRuns: []string{"logical_and_expression", "logical_or_expression"},
	Comments:    []string{"comment", "documentation_comment"},
	Owners:      []string{"class_definition", "mixin_declaration", "extension_declaration", "enum_declaration"},
	Imports:     []string{"library_import"},
	ParamKinds:  []string{"formal_parameter"},
	NameOf: func(n *tree_sitter.Node, src []byte) string {
		// method_signature holds the function_signature that holds the name.
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if name := n.NamedChild(i).ChildByFieldName("name"); name != nil {
				return name.Utf8Text(src)
			}
		}
		return ""
	},
}
