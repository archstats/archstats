package regex

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

var regexExtensions map[string]analysis.Extension

type embeddedExtensionDefinition struct {
	FileGlob  string   `yaml:"file_glob"`
	OnlyStats bool     `yaml:"only_stats"`
	Patterns  []string `yaml:"patterns"`
}
type ExtensionYamlFile struct {
	Extensions map[string]*embeddedExtensionDefinition `yaml:"extensions"`
}

func init() {
	regexExtensionsConfig := &ExtensionYamlFile{}
	yaml.Unmarshal(regexExtensionsRaw, regexExtensionsConfig)
	regexExtensions = make(map[string]analysis.Extension)
	for lang, extension := range regexExtensionsConfig.Extensions {
		var patterns []*regexp.Regexp
		for _, pattern := range extension.Patterns {
			patterns = append(patterns, regexp.MustCompile(pattern))
		}

		regexExtensions[lang] = &Extension{
			OnlyStats: extension.OnlyStats,
			Glob:      glob.MustCompile(extension.FileGlob),
			Patterns:  patterns,
		}

	}
}
func BuiltInRegexExtension(extension string) (analysis.Extension, error) {
	ext, has := regexExtensions[extension]
	if has {
		return ext, nil
	} else {
		return nil, errors.New("extension not found: " + extension)
	}
}
