package sqlite

import (
	"github.com/archstats/archstats/core"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Every table the exporter can write, and every _snapshot key, is described
// in DESCRIPTION.md. The document is how a person or an LLM learns the
// snapshot contract; it fell 13 views behind before this test existed.
func TestSchemaIsDocumented(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	doc, err := os.ReadFile(filepath.Join(root, "DESCRIPTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	documented := string(doc)

	names := registeredViewNames(t, filepath.Join(root, "extensions"), filepath.Join(root, "core"))
	if len(names) < 20 {
		t.Fatalf("found only %d registered views; the parser has stopped finding them", len(names))
	}
	for _, name := range names {
		if !strings.Contains(documented, "`"+name+"`") {
			t.Errorf("view %q is exported but not described in DESCRIPTION.md", name)
		}
	}
	for _, key := range core.KnownSnapshotKeys {
		if !strings.Contains(documented, "`"+key+"`") {
			t.Errorf("_snapshot key %q is written but not described in DESCRIPTION.md", key)
		}
	}
}

// registeredViewNames reads the Name of every core.ViewFactory literal in the
// given trees, resolving a name given as a constant within its package.
func registeredViewNames(t *testing.T, dirs ...string) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, dir := range dirs {
		consts := map[string]map[string]string{} // package dir -> const -> value
		var files []*ast.File
		var fileDirs []string
		fset := token.NewFileSet()
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, p, nil, 0)
			if err != nil {
				return err
			}
			files = append(files, f)
			fileDirs = append(fileDirs, filepath.Dir(p))
			for _, decl := range f.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				for _, spec := range gd.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, n := range vs.Names {
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								if consts[filepath.Dir(p)] == nil {
									consts[filepath.Dir(p)] = map[string]string{}
								}
								consts[filepath.Dir(p)][n.Name], _ = strconv.Unquote(lit.Value)
							}
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for i, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				sel, ok := cl.Type.(*ast.SelectorExpr)
				isFactory := ok && sel.Sel.Name == "ViewFactory"
				if id, ok := cl.Type.(*ast.Ident); ok && id.Name == "ViewFactory" {
					isFactory = true
				}
				if !isFactory {
					return true
				}
				for _, el := range cl.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok || kv.Key.(*ast.Ident).Name != "Name" {
						continue
					}
					switch v := kv.Value.(type) {
					case *ast.BasicLit:
						s, _ := strconv.Unquote(v.Value)
						seen[s] = true
					case *ast.Ident:
						if s, ok := consts[fileDirs[i]][v.Name]; ok {
							seen[s] = true
						} else {
							t.Errorf("view name %s in %s is not a string constant this test can read", v.Name, fileDirs[i])
						}
					}
				}
				return true
			})
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}
