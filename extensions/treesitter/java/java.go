package java

import (
	"fmt"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/treesitter/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

type Extension struct {
}

func (e *Extension) Init(settings core.Analyzer) error {
	settings.RegisterFileAnalyzer(createJavaLanguagePack())
	return nil
}

func createJavaLanguagePack() *common.LanguagePack {

	language := tree_sitter.NewLanguage(java.Language())
	var allQueries []string
	allQueries = append(allQueries, springQueries()...)
	allQueries = append(allQueries, jpaQueries()...)
	allQueries = append(allQueries, modularityQueries()...)

	lp := &common.LanguagePackTemplate{
		FileGlob: "**.java",
		Language: language,
		Queries:  allQueries,
	}
	template, err := common.PackFromTemplate(lp)
	if err != nil {
		panic(err)
	}
	return template
}

func springQueries() []string {
	return []string{
		createQueryForClassAnnotation("java__spring__controllers", "^(Controller|RestController)$"),
		createQueryForClassAnnotation("java__spring__services", "^Service$"),
		createQueryForClassAnnotation("java__spring__repositories", "^Repository$"),
		createQueryForClassAnnotation("java__spring__components", "^Component$"),
		createQueryForClassAnnotation("java__spring__configurations", "^Configuration$"),
		createQueryForClassAnnotation("java__spring__beans", "^(Component|Service|Repository|Controller|RestController|Configuration)$"),

		createQueryForMethodAnnotation("java__spring__request_mappings__total", "^(Request|Get|Put|Post|Delete|Patch)Mapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__get", "^GetMapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__put", "^PutMapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__post", "^PostMapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__delete", "^DeleteMapping$"),
		createQueryForMethodAnnotation("java__spring__request_mappings__patch", "^PatchMapping$"),
		createQueryForRequestMapping("java__spring__request_mappings__get", "GET"),
		createQueryForRequestMapping("java__spring__request_mappings__put", "PUT"),
		createQueryForRequestMapping("java__spring__request_mappings__post", "POST"),
		createQueryForRequestMapping("java__spring__request_mappings__delete", "DELETE"),
		createQueryForRequestMapping("java__spring__request_mappings__patch", "PATCH"),
	}
}
func jpaQueries() []string {
	return []string{
		createQueryForClassAnnotation("java__jpa__entity", "^Entity$"),
	}
}

func createQueryForRequestMapping(statName, method string) string {
	return fmt.Sprintf(`
((method_declaration (modifiers [
    	(annotation 
        	name: ((identifier) @_annotation_name) 
            arguments: 
            	(annotation_argument_list 
            		(element_value_pair 
                    	key: (identifier) @_argument
                        value: ([(identifier) (field_access)]) @_value
         ))) 
        (marker_annotation name: ((identifier)@_annotation_name))
        ] @%s) 
)
(#match? @_annotation_name "^RequestMapping$")
(#match? @_argument "^method$")
(#match? @_value "%s")
)
`, statName, method)
}

func createQueryForClassAnnotation(statName, annotationRegex string) string {
	return fmt.Sprintf(`
((class_declaration
	(modifiers [
    	(annotation name: ((identifier) @_annotation_name)) 
        (marker_annotation name: ((identifier)@_annotation_name))
        ] @%s) 
)
(#match? @_annotation_name "%s")
)`, statName, annotationRegex)
}

func createQueryForMethodAnnotation(statName, annotationRegex string) string {
	return fmt.Sprintf(`
((method_declaration
	(modifiers [
		(annotation name: ((identifier) @_annotation_name)) 
		(marker_annotation name: ((identifier)@_annotation_name))
		] @%s) 
)
(#match? @_annotation_name "%s")
)`, statName, annotationRegex)
}

func modularityQueries() []string {
	return []string{
		`(package_declaration  (scoped_identifier) @modularity__component__declarations)`,
		`
((interface_declaration name: (identifier) @modularity__types__total))
((class_declaration  name: (identifier) @modularity__types__total))
((record_declaration name: (identifier) @modularity__types__total))
`,
		`
((interface_declaration name: (identifier) @modularity__types__abstract))
((class_declaration ((modifiers) @_modifiers) name: (identifier) @modularity__types__abstract) (#match? @_modifiers "abstract"))
`,
		`
(
  ((import_declaration 
    ((scoped_identifier scope: (scoped_identifier) @modularity__component__imports))) @_import ) 
    (#not-match? @_import "(^import static)|[*]")
)
(
  ((import_declaration 
    ((scoped_identifier) @modularity__component__imports) (asterisk)) @_import)
    (#not-match? @_import "(^import static)")
    (#match? @_import "[*]")
)
(
  ((import_declaration
    ((scoped_identifier scope: (scoped_identifier) @modularity__component__imports))) @_import ) 
      (#match? @_import "(^import static)")
      (#not-match? @_import "[*]")
)
(
  ((import_declaration
      ((scoped_identifier) @modularity__component__imports) (asterisk)) @_import)
      (#match? @_import "(^import static)")
      (#match? @_import "[*]")
)
`,
	}
}
