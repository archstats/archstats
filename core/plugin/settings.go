package plugin

import (
	"fmt"
	"math"
	"strings"
)

// Values reads settings from a map, and an argument the map leaves out from
// the default its descriptor declares. A map decoded from JSON works as it
// is: whole float64s are read as ints and []any of strings as a string list.
func Values(values map[string]any, descriptors []*Descriptor) Settings {
	v := mapSettings{values: map[string]any{}}
	for _, d := range descriptors {
		for name, arg := range d.Arguments {
			if _, seen := v.values[name]; !seen {
				v.values[name] = arg.Default
			}
		}
	}
	for name, value := range values {
		v.values[name] = value
	}
	return v
}

type mapSettings struct {
	values map[string]any
}

func (m mapSettings) lookup(name string) (any, error) {
	value, ok := m.values[name]
	if !ok {
		return nil, fmt.Errorf("no setting named %q", name)
	}
	return value, nil
}

func (m mapSettings) GetString(name string) (string, error) {
	value, err := m.lookup(name)
	if err != nil {
		return "", err
	}
	if s, ok := value.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("setting %q is %T, not a string", name, value)
}

func (m mapSettings) GetInt(name string) (int, error) {
	value, err := m.lookup(name)
	if err != nil {
		return 0, err
	}
	switch n := value.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case float64:
		if n == math.Trunc(n) {
			return int(n), nil
		}
	}
	return 0, fmt.Errorf("setting %q is %v, not a whole number", name, value)
}

func (m mapSettings) GetBool(name string) (bool, error) {
	value, err := m.lookup(name)
	if err != nil {
		return false, err
	}
	if b, ok := value.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("setting %q is %T, not true or false", name, value)
}

func (m mapSettings) GetStringSlice(name string) ([]string, error) {
	value, err := m.lookup(name)
	if err != nil {
		return nil, err
	}
	switch s := value.(type) {
	case []string:
		return s, nil
	case string:
		// As the CLI reads a flag given once: comma-separated.
		if s == "" {
			return []string{}, nil
		}
		return strings.Split(s, ","), nil
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("setting %q holds %T, not only strings", name, item)
			}
			out = append(out, str)
		}
		return out, nil
	}
	return nil, fmt.Errorf("setting %q is %T, not a list of strings", name, value)
}
