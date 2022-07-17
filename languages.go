package main

import (
	"analyzer/regexsnippets"
	"analyzer/walker"
	"fmt"
	"regexp"
)

func getLanguageExtensions(lang string) []walker.SnippetProvider {
	if lang == "php" {
		return []walker.SnippetProvider{
			&regexsnippets.RegexBasedSnippetsProvider{
				Patterns: []*regexp.Regexp{
					regexp.MustCompile(fmt.Sprintf("namespace (?P<%s>.*);", walker.ComponentDeclaration)),
					regexp.MustCompile(fmt.Sprintf("(use|import) (?P<%s>.*)\\\\.*;", walker.ComponentImport)),
				},
			},
		}
	}
	return []walker.SnippetProvider{}
}
