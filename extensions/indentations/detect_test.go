package indentations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheWidthIsReadFromTheFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
		found   bool
	}{
		{"two spaces", "a:\n  b:\n    c: 1\n  d: 2\n", 2, true},
		{"four spaces", "class A {\n    void f() {\n        g();\n    }\n}\n", 4, true},
		{"tabs", "func f() {\n\tif x {\n\t\treturn\n\t}\n}\n", tabWidth, true},
		{"doc comments", "    /**\n     * doc\n     */\n    void f() {\n        g();\n    }\n", 4, true},
		{"a continuation does not outvote the blocks", "if (a) {\n  f(x,\n      y);\n  if (b) {\n    g();\n  }\n}\n", 2, true},
		{"flat", "a\nb\nc\n", 0, false},
		{"blank", "", 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := detectWidth([]byte(test.content))
			assert.Equal(t, test.found, found)
			assert.Equal(t, test.want, got)
		})
	}
}

// Read with four spaces a level, a two-space file came out half as deep.
func TestTwoAndFourSpaceFilesNestTheSame(t *testing.T) {
	two := analyze(Detected(), "if (a) {\n  if (b) {\n    if (c) {\n      f();\n    }\n  }\n}\n")
	four := analyze(Detected(), "if (a) {\n    if (b) {\n        if (c) {\n            f();\n        }\n    }\n}\n")
	tabs := analyze(Detected(), "if (a) {\n\tif (b) {\n\t\tif (c) {\n\t\t\tf();\n\t\t}\n\t}\n}\n")
	assert.Equal(t, 3, two)
	assert.Equal(t, 3, four)
	assert.Equal(t, 3, tabs)
}

func TestSpacesBeforeATabStillReachTheTab(t *testing.T) {
	assert.Equal(t, 1, leadingIndentation([]byte("  \tx"), 4))
	assert.Equal(t, 2, leadingIndentation([]byte("\t    x"), 4))
}

func TestTheProjectSaysBeforeTheFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".editorconfig", `root = true

[*]
indent_style = space
indent_size = 4

[*.{yml,yaml}]
indent_size = 2

[Makefile]
indent_style = tab

[legacy/**]
indent_size = unset
`)
	write(t, root, "web/.prettierrc", `{
	"tabWidth": 4,
	"overrides": [{ "files": "*.test.ts", "options": { "tabWidth": 8 } }]
}`)
	write(t, root, "app/package.json", `{"name": "app", "prettier": {"useTabs": false}}`)
	write(t, root, "lib/.prettierrc.js", `module.exports = { tabWidth: 3 }`)
	write(t, root, "lib/.editorconfig", "[*.ts]\nindent_size = 6\n")

	p := newProjectSettings()
	width := func(rel string) (int, bool) { return p.width(filepath.Join(root, rel)) }
	for _, test := range []struct {
		path  string
		want  int
		found bool
	}{
		{"main.go", 4, true},
		{"ci/deploy.yaml", 2, true},
		{"Makefile", tabWidth, true},
		{"legacy/old.c", 0, false},
		// Prettier's own config wins for the files it formats...
		{"web/src/a.ts", 4, true},
		{"web/src/a.test.ts", 8, true},
		// ...and not for the ones it does not.
		{"web/src/a.py", 4, true},
		// A config that does not say takes .editorconfig's word.
		{"app/src/b.ts", 4, true},
		// A config written in code cannot be read; .editorconfig still can.
		{"lib/c.ts", 6, true},
	} {
		got, found := width(test.path)
		assert.Equalf(t, test.found, found, test.path)
		assert.Equalf(t, test.want, got, test.path)
	}
}

func TestPrettierWithoutAWidthIndentsTwo(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".prettierrc.yaml", "semi: false\n")
	got, found := newProjectSettings().width(filepath.Join(root, "src", "a.tsx"))
	assert.True(t, found)
	assert.Equal(t, 2, got)
}

func analyze(ext *Extension, content string) int {
	for _, stat := range ext.AnalyzeFile(&fakeFile{content: []byte(content)}).Stats {
		if stat.StatType == Max {
			return stat.Value.(int)
		}
	}
	return -1
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	assert.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	assert.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}
