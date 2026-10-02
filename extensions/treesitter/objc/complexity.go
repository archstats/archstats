package objc

import (
	"strings"

	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// complexity reads Objective-C's methods, C functions and blocks for code
// health. A method is named by its selector: total:name:.
var complexity = &common.Complexity{
	Functions:  []string{"method_definition", "function_definition", "block_literal"},
	Ifs:        []string{"if_statement"},
	Elses:      []string{"else_clause"},
	Structures: []string{"for_statement", "while_statement", "do_statement", "switch_statement", "conditional_expression", "catch_clause"},
	Logical:    []string{"binary_expression"},
	Comments:   []string{"comment"},
	Owners:     []string{"class_implementation", "class_interface", "category_implementation"},
	Imports:    []string{"preproc_include", "module_import"},
	NameKinds:  []string{"identifier"},
	ParamKinds: []string{"method_parameter"},
	NameOf: func(n *tree_sitter.Node, src []byte) string {
		switch n.Kind() {
		case "method_definition":
			var parts []string
			params := false
			for i := uint(0); i < n.NamedChildCount(); i++ {
				switch c := n.NamedChild(i); c.Kind() {
				case "identifier":
					parts = append(parts, c.Utf8Text(src))
				case "method_parameter":
					params = true
				}
			}
			if params {
				return strings.Join(parts, ":") + ":"
			}
			return strings.Join(parts, "")
		case "function_definition":
			for d := n.ChildByFieldName("declarator"); d != nil; d = d.ChildByFieldName("declarator") {
				if d.Kind() == "identifier" {
					return d.Utf8Text(src)
				}
			}
		}
		return ""
	},
}
