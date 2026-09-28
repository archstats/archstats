package indentations

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"gopkg.in/yaml.v3"
)

// projectSettings answers what a project says its indentation is: its
// .editorconfig files and, for the files Prettier formats, its Prettier
// config. Both are read once per directory and shared by the file workers.
type projectSettings struct {
	mu            sync.Mutex
	editorconfigs map[string]*editorconfig
	prettiers     map[string]*prettierConfig
}

func newProjectSettings() *projectSettings {
	return &projectSettings{
		editorconfigs: map[string]*editorconfig{},
		prettiers:     map[string]*prettierConfig{},
	}
}

// width is the spaces per level the project declares for the file at
// absPath, and false when it declares none.
func (p *projectSettings) width(absPath string) (int, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if prettierFormats[strings.ToLower(filepath.Ext(absPath))] {
		if w, ok := p.prettierWidth(absPath); ok {
			return w, true
		}
	}
	return p.editorconfigWidth(absPath)
}

// matcher matches a path relative to a config file's directory. A pattern
// without a slash matches a file's name in any directory, as both
// EditorConfig and Prettier's overrides read it.
type matcher struct {
	g        glob.Glob
	baseName bool
}

func newMatcher(pattern string) (*matcher, bool) {
	pattern = strings.TrimSpace(pattern)
	baseName := !strings.Contains(pattern, "/")
	g, err := glob.Compile(strings.TrimPrefix(pattern, "/"), '/')
	if err != nil {
		return nil, false
	}
	return &matcher{g: g, baseName: baseName}, true
}

func (m *matcher) matches(rel string) bool {
	if m.baseName {
		return m.g.Match(path.Base(rel))
	}
	return m.g.Match(rel)
}

func relativeTo(dir, absPath string) (string, bool) {
	rel, err := filepath.Rel(dir, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// EditorConfig, https://spec.editorconfig.org

type editorconfig struct {
	dir      string
	root     bool
	sections []editorconfigSection
}

type editorconfigSection struct {
	match *matcher
	props map[string]string
}

// editorconfigWidth applies every .editorconfig from the file's directory up
// to the one marked root, the nearer overriding the farther, and reads the
// indentation they settle on.
func (p *projectSettings) editorconfigWidth(absPath string) (int, bool) {
	props := p.editorconfigProps(absPath)
	size, style := props["indent_size"], props["indent_style"]
	tabs, tabsOK := positive(props["tab_width"])
	if n, ok := positive(size); ok && style != "tab" {
		return n, true
	}
	if style == "tab" || size == "tab" {
		if tabsOK {
			return tabs, true
		}
		if n, ok := positive(size); ok {
			return n, true
		}
		return tabWidth, true
	}
	if style == "space" && tabsOK {
		return tabs, true
	}
	return 0, false
}

func (p *projectSettings) editorconfigProps(absPath string) map[string]string {
	var chain []*editorconfig
	for dir := filepath.Dir(absPath); ; {
		if ec := p.editorconfig(dir); ec != nil {
			chain = append(chain, ec)
			if ec.root {
				break
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	props := map[string]string{}
	for i := len(chain) - 1; i >= 0; i-- {
		rel, ok := relativeTo(chain[i].dir, absPath)
		if !ok {
			continue
		}
		for _, section := range chain[i].sections {
			if !section.match.matches(rel) {
				continue
			}
			for k, v := range section.props {
				if v == "unset" {
					delete(props, k)
				} else {
					props[k] = v
				}
			}
		}
	}
	return props
}

func (p *projectSettings) editorconfig(dir string) *editorconfig {
	if ec, seen := p.editorconfigs[dir]; seen {
		return ec
	}
	ec := readEditorconfig(dir)
	p.editorconfigs[dir] = ec
	return ec
}

func readEditorconfig(dir string) *editorconfig {
	content, err := os.ReadFile(filepath.Join(dir, ".editorconfig"))
	if err != nil {
		return nil
	}
	ec := &editorconfig{dir: dir}
	var current *editorconfigSection
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			current = nil
			if m, ok := newMatcher(line[1 : len(line)-1]); ok {
				ec.sections = append(ec.sections, editorconfigSection{match: m, props: map[string]string{}})
				current = &ec.sections[len(ec.sections)-1]
			}
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.ToLower(strings.TrimSpace(value))
		switch {
		case current != nil:
			current.props[key] = value
		case key == "root" && len(ec.sections) == 0:
			ec.root = value == "true"
		}
	}
	return ec
}

// Prettier, https://prettier.io/docs/en/configuration

// prettierFormats are the files Prettier formats without a plugin.
var prettierFormats = map[string]bool{
	".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".ts": true, ".tsx": true, ".mts": true, ".cts": true,
	".vue": true, ".html": true, ".css": true, ".scss": true, ".less": true,
	".json": true, ".json5": true, ".md": true, ".mdx": true, ".yaml": true, ".yml": true,
	".graphql": true, ".gql": true, ".hbs": true, ".handlebars": true,
}

// Where Prettier looks for its config, nearest directory first. Only the
// JSON and YAML forms can be read without running the project's code.
var prettierFiles = []string{
	"package.json", ".prettierrc", ".prettierrc.json", ".prettierrc.yaml", ".prettierrc.yml",
	".prettierrc.json5", ".prettierrc.js", ".prettierrc.cjs", ".prettierrc.mjs", ".prettierrc.ts",
	".prettierrc.cts", ".prettierrc.mts", "prettier.config.js", "prettier.config.cjs",
	"prettier.config.mjs", "prettier.config.ts", "prettier.config.cts", "prettier.config.mts",
	".prettierrc.toml",
}

type prettierOptions struct {
	TabWidth *int  `yaml:"tabWidth"`
	UseTabs  *bool `yaml:"useTabs"`
}

type prettierOverride struct {
	Files        stringList      `yaml:"files"`
	ExcludeFiles stringList      `yaml:"excludeFiles"`
	Options      prettierOptions `yaml:"options"`
}

type prettierConfig struct {
	dir string
	// readable is false for a config written in code, or one that only names
	// a shared config: Prettier is there, its settings are not known.
	readable  bool
	options   prettierOptions
	overrides []prettierOverride
}

// prettierWidth reads the nearest Prettier config. Prettier takes what its
// own config leaves unsaid from .editorconfig, and 2 after that.
func (p *projectSettings) prettierWidth(absPath string) (int, bool) {
	cfg := p.prettier(filepath.Dir(absPath))
	if cfg == nil || !cfg.readable {
		return 0, false
	}
	options := cfg.options
	if rel, ok := relativeTo(cfg.dir, absPath); ok {
		for _, o := range cfg.overrides {
			if o.Files.matches(rel) && !o.ExcludeFiles.matches(rel) {
				if o.Options.TabWidth != nil {
					options.TabWidth = o.Options.TabWidth
				}
				if o.Options.UseTabs != nil {
					options.UseTabs = o.Options.UseTabs
				}
			}
		}
	}
	if options.TabWidth != nil && *options.TabWidth > 0 {
		return *options.TabWidth, true
	}
	if w, ok := p.editorconfigWidth(absPath); ok {
		return w, true
	}
	return 2, true
}

func (p *projectSettings) prettier(dir string) *prettierConfig {
	if cfg, seen := p.prettiers[dir]; seen {
		return cfg
	}
	cfg := readPrettier(dir)
	if cfg == nil {
		if parent := filepath.Dir(dir); parent != dir {
			cfg = p.prettier(parent)
		}
	}
	p.prettiers[dir] = cfg
	return cfg
}

func readPrettier(dir string) *prettierConfig {
	for _, name := range prettierFiles {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		switch name {
		case "package.json":
			var pkg struct {
				Prettier json.RawMessage `json:"prettier"`
			}
			if json.Unmarshal(content, &pkg) != nil || len(pkg.Prettier) == 0 {
				continue // a package.json without a "prettier" key is not a config
			}
			content = pkg.Prettier
			if content[0] != '{' {
				return &prettierConfig{dir: dir}
			}
		case ".prettierrc", ".prettierrc.json", ".prettierrc.yaml", ".prettierrc.yml", ".prettierrc.json5":
		default:
			return &prettierConfig{dir: dir}
		}
		// YAML reads JSON too, and .prettierrc is either. YAML refuses the
		// tabs a JSON file may be indented with, so they are read as spaces.
		var raw struct {
			prettierOptions `yaml:",inline"`
			Overrides       []prettierOverride `yaml:"overrides"`
		}
		if yaml.Unmarshal(bytes.ReplaceAll(content, []byte("\t"), []byte("  ")), &raw) != nil {
			return &prettierConfig{dir: dir}
		}
		return &prettierConfig{dir: dir, readable: true, options: raw.prettierOptions, overrides: raw.Overrides}
	}
	return nil
}

// stringList is Prettier's "one pattern or a list of them".
type stringList []string

func (s *stringList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*s = []string{node.Value}
		return nil
	}
	var list []string
	err := node.Decode(&list)
	*s = list
	return err
}

func (s stringList) matches(rel string) bool {
	for _, pattern := range s {
		if m, ok := newMatcher(pattern); ok && m.matches(rel) {
			return true
		}
	}
	return false
}

func positive(value string) (int, bool) {
	n, err := strconv.Atoi(value)
	return n, err == nil && n > 0
}
