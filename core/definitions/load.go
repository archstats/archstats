package definitions

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/fs"
	"strings"
)

func LoadYamlFiles(fsys fs.ReadFileFS) ([]*Definition, error) {
	var definitions []*Definition

	// A definition that fails to load stops the load with its name. The
	// walk's error used to be dropped, so one malformed file silently lost
	// every definition after it.
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			def, err := LoadYamlFile(fsys, path)
			if err != nil {
				return fmt.Errorf("definition %s: %w", path, err)
			}
			definitions = append(definitions, def)

		}
		return nil
	})
	if err != nil {
		return nil, err
	}
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
