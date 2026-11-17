package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/archstats/archstats/core/walker"
	"github.com/rs/zerolog"
)

// No file stops a scan. The engine runs inside the desktop app, and every
// crash it has had came from a file nobody planned for: a NUL byte in
// source, a manifest in a shape its reader never saw, a query that matched
// something it did not expect. The walker recovers a panic and skips the
// file, which keeps the app alive but hides the bug, so this fails on a
// recovered panic as well as on an error.
//
// One file per input, named so that discovery switches on the language pack,
// manifest reader, import resolver or CI reader that file belongs to; then
// every view is rendered. `go test` runs the seeds below. To search further:
//
//	go test ./engine -run '^$' -fuzz FuzzScan -fuzztime 2m
//
// A failing input lands in testdata/fuzz/FuzzScan and is a seed from then on:
// commit it with the fix.
var fuzzNames = []string{
	"src/Main.java", "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle",
	"src/main.kt", "src/Program.cs", "App.csproj", "App.sln",
	"main.go", "go.mod", "app/app.py", "app/settings.py", "pyproject.toml", "requirements.txt",
	"src/index.ts", "src/index.js", "package.json", "tsconfig.json",
	"src/index.php", "composer.json", "Sources/App/main.swift", "Package.swift",
	"lib/main.dart", "pubspec.yaml", "App/AppDelegate.m",
	"schema.prisma", "Dockerfile", ".github/workflows/ci.yml", ".gitlab-ci.yml", "Jenkinsfile",
	"AndroidManifest.xml", "README.md", "CODEOWNERS",
}

func FuzzScan(f *testing.F) {
	plausible := map[string]string{
		"src/Main.java":            "package a.b;\nimport a.c.D;\n@Service\npublic class Main extends D implements E { @GetMapping(\"/x\") void m() { if (x) { for(;;) {} } } }\n",
		"pom.xml":                  "<project><groupId>g</groupId><artifactId>a</artifactId><modules><module>m</module></modules><dependencies><dependency><groupId>g</groupId><artifactId>b</artifactId></dependency></dependencies></project>",
		"build.gradle.kts":         "plugins { kotlin(\"jvm\") }\ndependencies { implementation(project(\":core\")) }\n",
		"src/main.kt":              "package a\nimport b.C\n@Component class Main(val c: C) { fun x() = when (c) { else -> 1 } }\n",
		"src/Program.cs":           "namespace A.B;\nusing C.D;\npublic partial class P : Base { [HttpGet(\"x\")] public void M() { if (a) {} } }\n",
		"App.csproj":               "<Project Sdk=\"Microsoft.NET.Sdk\"><ItemGroup><ProjectReference Include=\"..\\Core\\Core.csproj\" /></ItemGroup></Project>",
		"main.go":                  "package main\n\nimport \"example.com/m/store\"\n\nfunc main() { store.Open() }\n",
		"go.mod":                   "module example.com/m\n\ngo 1.23\n\nrequire example.com/x v1.0.0\nreplace example.com/x => ../x\n",
		"app/app.py":               "from . import models\nfrom ..core import x as y\nimport os\n@app.route('/x')\ndef f():\n    if a:\n        return 1\n",
		"package.json":             "{\"name\":\"a\",\"workspaces\":[\"packages/*\"],\"dependencies\":{\"b\":\"workspace:*\"}}",
		"src/index.ts":             "import { a } from '@/x';\nconst b = require('./b');\nexport default class C { @Get('x') m() { return import('./d'); } }\n",
		"src/index.php":            "<?php\nnamespace A\\B;\nuse C\\D;\n#[Route('/x')]\nclass E extends D { public function f() { if ($a) {} } }\n",
		"composer.json":            "{\"name\":\"a/b\",\"autoload\":{\"psr-4\":{\"A\\\\\":\"src/\"}}}",
		"Package.swift":            "// swift-tools-version:5.9\nimport PackageDescription\nlet package = Package(name: \"A\", targets: [.target(name: \"A\", dependencies: [\"B\"])])\n",
		"Sources/App/main.swift":   "import Foundation\nstruct A: View { var body: some View { Text(\"x\") } }\n",
		"lib/main.dart":            "import 'package:a/b.dart';\nclass A extends StatelessWidget { Widget build(c) => Text('x'); }\n",
		"pubspec.yaml":             "name: a\ndependencies:\n  b:\n    path: ../b\n",
		"schema.prisma":            "model User {\n  id Int @id\n  posts Post[]\n}\n",
		"Dockerfile":               "FROM golang:1.23 AS build\nCOPY . .\nRUN go build -o /x .\nFROM scratch\nCOPY --from=build /x /x\n",
		".github/workflows/ci.yml": "on: [push]\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - run: make\n",
	}
	// Shapes that broke a reader before, applied to every name.
	hostile := []string{"", "\x00", "a\x00b", "\xff\xfe\xfd", strings.Repeat("{", 2000), strings.Repeat("(", 500) + strings.Repeat(")", 499), "\r\n\r\n\t\t"}
	for i, name := range fuzzNames {
		if p, ok := plausible[name]; ok {
			f.Add(uint8(i), []byte(p))
		}
	}
	for i := range fuzzNames {
		f.Add(uint8(i), []byte(hostile[i%len(hostile)]))
	}

	level := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	f.Cleanup(func() { zerolog.SetGlobalLevel(level) })

	f.Fuzz(func(t *testing.T, which uint8, content []byte) {
		name := fuzzNames[int(which)%len(fuzzNames)]
		root := t.TempDir()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}

		results, err := Analyze(Options{Root: root})
		if err != nil {
			t.Fatalf("%s: scan failed: %v", name, err)
		}
		for _, s := range results.SkippedFiles {
			if s.Reason == walker.SkipFailed {
				t.Fatalf("%s: analysing it panicked: %s", name, s.Detail)
			}
		}
		for _, factory := range results.GetViewFactories() {
			if _, err := results.RenderView(factory.Name); err != nil {
				t.Fatalf("%s: view %s: %v", name, factory.Name, err)
			}
		}
	})
}
