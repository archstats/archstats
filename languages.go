package main

import (
	"archstats/snippets"
	"fmt"
	"regexp"
)

func getLanguageExtensions(lang string) []snippets.SnippetProvider {
	if lang == "php" {
		return []snippets.SnippetProvider{
			&snippets.RegexBasedSnippetsProvider{
				Patterns: []*regexp.Regexp{
					regexp.MustCompile(fmt.Sprintf("namespace (?P<%s>.*);", snippets.ComponentDeclaration)),
					regexp.MustCompile(fmt.Sprintf("(use|import) (?P<%s>.*)\\\\.*;", snippets.ComponentImport)),
				},
			},
		}
	}
	return []snippets.SnippetProvider{}
}
