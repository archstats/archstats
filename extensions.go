package main

import (
	"archstats/snippets"
	"fmt"
	"regexp"
)

func getExtensions(lang string) []snippets.SnippetProvider {
	if lang == "php" {
		return []snippets.SnippetProvider{
			&snippets.RegexBasedSnippetsProvider{
				Patterns: []*regexp.Regexp{
					regexp.MustCompile(fmt.Sprintf("namespace (?P<%s>.*);", snippets.ComponentDeclaration)),
					regexp.MustCompile(fmt.Sprintf("(use|import) (?P<%s>.*)\\\\.*;", snippets.ComponentImport)),
					regexp.MustCompile(fmt.Sprintf("(abstract class|interface|trait) (?P<%s>\\w+)", snippets.AbstractType)),
					regexp.MustCompile(fmt.Sprintf("(class|interface|trait) (?P<%s>\\w+)", snippets.Type)),
				},
			},
		}
	}
	return []snippets.SnippetProvider{}
}
