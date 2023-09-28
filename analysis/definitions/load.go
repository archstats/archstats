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
			def, err := LoadYamlFile(fsys, path)
			if err != nil {
				return err
			}
			definitions = append(definitions, def)

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

	var definitions Definition

	err = yaml.Unmarshal(file, &definitions)
	if err != nil {
		return nil, err
	}

	return &definitions, nil
}
