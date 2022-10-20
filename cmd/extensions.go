package cmd

import (
	_ "embed"
	"errors"
	"github.com/RyanSusana/archstats/analysis"
	"github.com/gobwas/glob"
	"gopkg.in/yaml.v3"
	"regexp"
)

//go:embed regex_extensions.yaml
var regexExtensionsRaw []byte

var regexExtensions map[string]analysis.SnippetProvider

type embeddedExtensionDefinition struct {
	FileGlob string   `yaml:"file_glob"`
	Patterns []string `yaml:"patterns"`
}
type RegexExtensions struct {
	Extensions map[string]*embeddedExtensionDefinition `yaml:"extensions"`
}

func init() {
	regexExtensionsConfig := &RegexExtensions{}
	yaml.Unmarshal(regexExtensionsRaw, regexExtensionsConfig)
	regexExtensions = make(map[string]analysis.SnippetProvider)
	for lang, extension := range regexExtensionsConfig.Extensions {
		var patterns []*regexp.Regexp
		for _, pattern := range extension.Patterns {
			patterns = append(patterns, regexp.MustCompile(pattern))
		}
		regexExtensions[lang] = &analysis.RegexBasedSnippetsProvider{
			Glob:     glob.MustCompile(extension.FileGlob),
			Patterns: patterns,
		}
	}
}
func getExtension(extension string) (analysis.SnippetProvider, error) {
	ext, has := regexExtensions[extension]
	if has {
		return ext, nil
	} else {
		return nil, errors.New("extension not found: " + extension)
	}
}
