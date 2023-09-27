package definitions

import (
	"gopkg.in/yaml.v3"
	"io/fs"
	"strings"
)

func LoadYamlFiles(fsys fs.ReadFileFS) ([]*Definition, error) {
	var definitions []*Definition

	fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			definition, err := LoadYamlFile(fsys, path)
			if err != nil {
				return err
			}
			definitions = append(definitions, definition)

		}
		return nil
	})
	return definitions, nil
}

func LoadYamlFile(fsys fs.ReadFileFS, path string) (*Definition, error) {
	file, err := fsys.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var definition Definition

	err = yaml.Unmarshal(file, &definition)
	if err != nil {
		return nil, err
	}

	return &definition, nil
}

func LoadYaml(file fs.File) (*Definition, error) {

	return nil, nil
}
