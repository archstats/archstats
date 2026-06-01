package declbased

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
)

func TestComponentLinker_DeclaredStrategy(t *testing.T) {
	linker := &componentLinker{Strategy: "declared"}

	fileResults := []*file.Results{
		{
			Name:      "src/foo/File1.java",
			Directory: "src/foo",
			Snippets: []*file.Snippet{
				{File: "src/foo/File1.java", Type: file.ComponentDeclaration, Value: "com.foo"},
				{File: "src/foo/File1.java", Type: file.Type, Value: "MyClass"},
			},
		},
		{
			Name:      "src/bar/File2.java",
			Directory: "src/bar",
			Snippets: []*file.Snippet{
				{File: "src/bar/File2.java", Type: file.Type, Value: "AnotherClass"},
			},
		},
	}

	linker.EditFileResults(fileResults)

	// In File1, the total type is mapped to com.foo
	assert.Equal(t, "com.foo", fileResults[0].Snippets[1].Component)

	// In File2, the total type is mapped to Unknown
	assert.Equal(t, "Unknown", fileResults[1].Snippets[0].Component)
}

func TestComponentLinker_JS_TS_RelativeImports(t *testing.T) {
	linker := &componentLinker{Strategy: "directory"}

	fileResults := []*file.Results{
		{
			Name:      "src/components/button/button.ts",
			Directory: "src/components/button",
			Snippets: []*file.Snippet{
				{File: "src/components/button/button.ts", Type: file.Type, Value: "ButtonClass"},
				// Sibling imports
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "./types"},
				// Subfolder imports
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "./utils/helpers"},
				// Parent folder imports
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "../layout"},
				// Grandparent folder imports
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "../../utils/logger"},
				// Third-party / npm imports (should remain unchanged)
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "react"},
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "lodash/fp"},
				// Windows backslash imports
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: ".\\utils\\styles"},
				// TS/JS Alias path mappings
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "@/components/button/button"},
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "~/utils/helpers"},
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "@components/layout"},
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "utils/logger"},
			},
		},
		{
			Name:      "src/components/layout/layout.ts",
			Directory: "src/components/layout",
		},
		{
			Name:      "src/utils/logger/logger.ts",
			Directory: "src/utils/logger",
		},
		{
			Name:      "src/components/button/utils/helpers.ts",
			Directory: "src/components/button/utils",
		},
	}

	linker.EditFileResults(fileResults)

	buttonSnippets := fileResults[0].Snippets

	// 1. Sibling import: "./types" from "src/components/button" -> should resolve to "src/components/button" (since resolved is "src/components/button/types" and parent "src/components/button" is a known folder containing files)
	assert.Equal(t, "src/components/button", buttonSnippets[1].Value)

	// 2. Subfolder import: "./utils/helpers" -> points to "src/components/button/utils/helpers", resolves to its directory: "src/components/button/utils"
	assert.Equal(t, "src/components/button/utils", buttonSnippets[2].Value)

	// 3. Parent folder import: "../layout" -> points to "src/components/layout", which exists in our known directories
	assert.Equal(t, "src/components/layout", buttonSnippets[3].Value)

	// 4. Grandparent folder import: "../../utils/logger" -> points to "src/utils/logger", which exists in our known directories
	assert.Equal(t, "src/utils/logger", buttonSnippets[4].Value)

	// 5. Third-party imports: "react" and "lodash/fp" should not be resolved relative to local folders
	assert.Equal(t, "react", buttonSnippets[5].Value)
	assert.Equal(t, "lodash/fp", buttonSnippets[6].Value)

	// 6. Windows backslashes: ".\\utils\\styles" -> resolved and cleaned to "src/components/button/utils"
	assert.Equal(t, "src/components/button/utils", buttonSnippets[7].Value)

	// 7. TS path alias "@/": "@/components/button/button" -> should resolve to "src/components/button"
	assert.Equal(t, "src/components/button", buttonSnippets[8].Value)

	// 8. TS path alias "~/": "~/utils/helpers" -> should resolve to "src/components/button/utils"
	assert.Equal(t, "src/components/button/utils", buttonSnippets[9].Value)

	// 9. TS path alias without slash "@": "@components/layout" -> should resolve to "src/components/layout"
	assert.Equal(t, "src/components/layout", buttonSnippets[10].Value)

	// 10. Absolute-like path: "utils/logger" -> should resolve to "src/utils/logger"
	assert.Equal(t, "src/utils/logger", buttonSnippets[11].Value)
}

func TestComponentLinker_Python_DotImports(t *testing.T) {
	linker := &componentLinker{Strategy: "directory"}

	fileResults := []*file.Results{
		{
			Name:      "my_app/services/auth.py",
			Directory: "my_app/services",
			Snippets: []*file.Snippet{
				{File: "my_app/services/auth.py", Type: file.Type, Value: "AuthService"},
				// Package-level import
				{File: "my_app/services/auth.py", Type: file.ComponentImport, Value: "my_app"},
				// Sibling/Nested import
				{File: "my_app/services/auth.py", Type: file.ComponentImport, Value: "my_app.services.db"},
				// Import from parent / another package
				{File: "my_app/services/auth.py", Type: file.ComponentImport, Value: "my_app.utils.crypto"},
				// Unresolved third-party import (mapped to slashed format)
				{File: "my_app/services/auth.py", Type: file.ComponentImport, Value: "numpy.random.random"},
				// Python relative dot imports
				{File: "my_app/services/auth.py", Type: file.ComponentImport, Value: ".db"},
				{File: "my_app/services/auth.py", Type: file.ComponentImport, Value: "..utils.crypto"},
				{File: "my_app/services/auth.py", Type: file.ComponentImport, Value: ".."},
			},
		},
		{
			Name:      "my_app/services/db.py",
			Directory: "my_app/services",
		},
		{
			Name:      "my_app/utils/crypto.py",
			Directory: "my_app/utils",
		},
	}

	linker.EditFileResults(fileResults)

	authSnippets := fileResults[0].Snippets

	// 1. Package-level import "my_app" -> converted to directory "my_app", matches known folders
	assert.Equal(t, "my_app", authSnippets[1].Value)

	// 2. Sibling/Nested import "my_app.services.db" -> converted to "my_app/services/db", resolves to "my_app/services" directory
	assert.Equal(t, "my_app/services", authSnippets[2].Value)

	// 3. Parent/Utility import "my_app.utils.crypto" -> converted to "my_app/utils/crypto", resolves to "my_app/utils" directory
	assert.Equal(t, "my_app/utils", authSnippets[3].Value)

	// 4. Third-party import "numpy.random.random" -> mapped to slashed "numpy/random/random" (since not matched in known directories)
	assert.Equal(t, "numpy/random/random", authSnippets[4].Value)

	// 5. Python relative dot sibling: ".db" -> should resolve to "my_app/services"
	assert.Equal(t, "my_app/services", authSnippets[5].Value)

	// 6. Python relative dot parent sibling: "..utils.crypto" -> should resolve to "my_app/utils"
	assert.Equal(t, "my_app/utils", authSnippets[6].Value)

	// 7. Python relative dot parent only: ".." -> should resolve to "my_app"
	assert.Equal(t, "my_app", authSnippets[7].Value)
}

func TestComponentLinker_FallbackStrategy_Detailed(t *testing.T) {
	linker := &componentLinker{Strategy: "fallback"}

	fileResults := []*file.Results{
		{
			Name:      "src/foo/File1.java",
			Directory: "src/foo",
			Snippets: []*file.Snippet{
				{File: "src/foo/File1.java", Type: file.ComponentDeclaration, Value: "com.declared.foo"},
				{File: "src/foo/File1.java", Type: file.Type, Value: "MyClass"},
				// relative import
				{File: "src/foo/File1.java", Type: file.ComponentImport, Value: "../bar"},
				// dot import
				{File: "src/foo/File1.java", Type: file.ComponentImport, Value: "com.declared.bar"},
			},
		},
		{
			Name:      "src/bar/File2.java",
			Directory: "src/bar",
			Snippets: []*file.Snippet{
				{File: "src/bar/File2.java", Type: file.Type, Value: "AnotherClass"},
			},
		},
		{
			Name:      "src/baz/File3.java",
			Directory: "src/baz",
			Snippets: []*file.Snippet{
				{File: "src/baz/File3.java", Type: file.ComponentDeclaration, Value: "com.declared.bar"},
				{File: "src/baz/File3.java", Type: file.Type, Value: "BarClass"},
			},
		},
	}

	linker.EditFileResults(fileResults)

	fooSnippets := fileResults[0].Snippets

	// File1 has explicit declaration, resolves to com.declared.foo
	assert.Equal(t, "com.declared.foo", fooSnippets[1].Component)

	// File2 has no declaration, resolves to fallback directory "src/bar"
	assert.Equal(t, "src/bar", fileResults[1].Snippets[0].Component)

	// File3 has explicit declaration, resolves to com.declared.bar
	assert.Equal(t, "com.declared.bar", fileResults[2].Snippets[1].Component)

	// JS relative import resolved to "src/bar"
	assert.Equal(t, "src/bar", fooSnippets[2].Value)

	// Java/Python dot import resolved to "com.declared.bar" (matches declared component)
	assert.Equal(t, "com.declared.bar", fooSnippets[3].Value)
}

func TestComponentLinker_Python_MultiLevelRelativeImports(t *testing.T) {
	linker := &componentLinker{Strategy: "directory"}

	fileResults := []*file.Results{
		{
			Name:      "my_app/services/sub/nested/auth.py",
			Directory: "my_app/services/sub/nested",
			Snippets: []*file.Snippet{
				{File: "my_app/services/sub/nested/auth.py", Type: file.Type, Value: "NestedAuth"},
				// 3 dots: go up 2 levels -> from "my_app/services/sub/nested" to "my_app/services", then join with "db" -> "my_app/services/db"
				{File: "my_app/services/sub/nested/auth.py", Type: file.ComponentImport, Value: "...db"},
				// 4 dots: go up 3 levels -> from "my_app/services/sub/nested" to "my_app", then join with "utils.crypto" -> "my_app/utils/crypto"
				{File: "my_app/services/sub/nested/auth.py", Type: file.ComponentImport, Value: "....utils.crypto"},
			},
		},
		{
			Name:      "my_app/services/db.py",
			Directory: "my_app/services",
		},
		{
			Name:      "my_app/utils/crypto.py",
			Directory: "my_app/utils",
		},
	}

	linker.EditFileResults(fileResults)

	snippets := fileResults[0].Snippets
	// 3 dots resolved from "my_app/services/sub/nested" -> parent (sub) -> grandparent (services), joins db -> "my_app/services"
	assert.Equal(t, "my_app/services", snippets[1].Value)

	// 4 dots resolved from "my_app/services/sub/nested" -> parent (sub) -> grandparent (services) -> great-grandparent (my_app), joins utils/crypto -> "my_app/utils"
	assert.Equal(t, "my_app/utils", snippets[2].Value)
}

func TestComponentLinker_JS_TS_Alias_Ambiguation(t *testing.T) {
	linker := &componentLinker{Strategy: "directory"}

	fileResults := []*file.Results{
		{
			Name:      "src/components/button/button.ts",
			Directory: "src/components/button",
			Snippets: []*file.Snippet{
				{File: "src/components/button/button.ts", Type: file.Type, Value: "ButtonClass"},
				// Both src/components/button/utils and src/utils contain "utils"
				// An import of "@/components/button/utils" matches "src/components/button/utils"
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "@/components/button/utils"},
				// An import of "@/utils" matches "src/utils"
				{File: "src/components/button/button.ts", Type: file.ComponentImport, Value: "@/utils"},
			},
		},
		{
			Name:      "src/components/button/utils/helper.ts",
			Directory: "src/components/button/utils",
		},
		{
			Name:      "src/utils/logger.ts",
			Directory: "src/utils",
		},
	}

	linker.EditFileResults(fileResults)

	snippets := fileResults[0].Snippets
	// Path mapping "@/" resolves to "src/components/button/utils" since it matches the longer suffix
	assert.Equal(t, "src/components/button/utils", snippets[1].Value)

	// Path mapping "@/" resolves to "src/utils"
	assert.Equal(t, "src/utils", snippets[2].Value)
}
