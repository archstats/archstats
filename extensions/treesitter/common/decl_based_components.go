package common

import (
	"github.com/archstats/archstats/core/file"
	"github.com/samber/lo"
)

func DeclarationBasedComponentResolution(r *file.Results) string {
	componentDeclarations := lo.Filter(r.Snippets, func(snippet *file.Snippet, idx int) bool {
		return snippet.Type == file.ComponentDeclaration
	})
	if len(componentDeclarations) == 0 {
		return "Unknown"
	}
	return componentDeclarations[0].Value
}
