package main

import (
	"analyzer/core"
	"analyzer/regexsnippets"
	"fmt"
	"regexp"
)

func getLanguageExtensions(lang string) []core.SnippetProvider {
	if lang == "php" {
		return []core.SnippetProvider{
			&regexsnippets.RegexBasedSnippetsProvider{
				Patterns: []*regexp.Regexp{
					regexp.MustCompile(fmt.Sprintf("namespace (?P<%s>.*);", core.ComponentDeclaration)),
					regexp.MustCompile(fmt.Sprintf("(use|import) (?P<%s>.*)\\\\.*;", core.ComponentImport)),
				},
			},
		}
	}
	return []core.SnippetProvider{}
}
