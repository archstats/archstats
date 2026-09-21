package typescript

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
)

func TestTypeScriptLanguagePack_TSX(t *testing.T) {
	pack := createTypeScriptLanguagePack(true) // TSX dialect

	content := `
import * as React from "react";
import { Button } from "./Button";

export interface ButtonProps {
  label: string;
}

export abstract class BaseButton {}

export function TSXComponent() {
  const state = useMyCustomHook();
  return null;
}
`

	results := pack.AnalyzeFileContent("src/Button.tsx", []byte(content))
	assert.NotNil(t, results)

	// Check total and abstract types
	var totalTypes []string
	var abstractTypes []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "modularity__types__total" {
			totalTypes = append(totalTypes, snippet.Value)
		}
		if snippet.Type == "modularity__types__abstract" {
			abstractTypes = append(abstractTypes, snippet.Value)
		}
	}
	assert.Contains(t, totalTypes, "ButtonProps")
	assert.Contains(t, totalTypes, "BaseButton")
	assert.Contains(t, abstractTypes, "ButtonProps")
	assert.Contains(t, abstractTypes, "BaseButton")

	// Check React component & Hook
	var reactComponents []string
	var reactHooks []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "ts__react__components" {
			reactComponents = append(reactComponents, snippet.Value)
		}
		if snippet.Type == "ts__react__hooks" {
			reactHooks = append(reactHooks, snippet.Value)
		}
	}
	assert.Contains(t, reactComponents, "TSXComponent")
	assert.Contains(t, reactHooks, "useMyCustomHook")
}

func TestTypeScriptLanguagePack_Angular(t *testing.T) {
	pack := createTypeScriptLanguagePack(false) // TS dialect

	content := `
import { Component, Injectable, Directive, Pipe } from '@angular/core';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html'
})
export class AppComponent {}

@Injectable({
  providedIn: 'root'
})
export class DataService {}

@Directive({
  selector: '[appHighlight]'
})
export class HighlightDirective {}

@Pipe({
  name: 'exponentialStrength'
})
export class ExponentialStrengthPipe {}
`

	results := pack.AnalyzeFileContent("src/app.component.ts", []byte(content))
	assert.NotNil(t, results)

	// Check Angular components, services, directives, pipes
	var components []string
	var services []string
	var directives []string
	var pipes []string

	for _, snippet := range results.Snippets {
		switch snippet.Type {
		case "ts__angular__components":
			components = append(components, snippet.Value)
		case "ts__angular__services":
			services = append(services, snippet.Value)
		case "ts__angular__directives":
			directives = append(directives, snippet.Value)
		case "ts__angular__pipes":
			pipes = append(pipes, snippet.Value)
		}
	}

	assert.Contains(t, components, "Component")
	assert.Contains(t, services, "Injectable")
	assert.Contains(t, directives, "Directive")
	assert.Contains(t, pipes, "Pipe")
}

// `import type` is erased by the compiler. It is a dependency on a shape and
// none at all at runtime, and LibreChat writes 681 of its 5,100 imports that
// way -- an eighth of the graph, if they are counted as coupling.
func TestTypeOnlyImports(t *testing.T) {
	pack := createTypeScriptLanguagePack(false)
	src := `
import type { Config } from "./config";
import type Thing from "./thing";
import { type Inline, run } from "./runner";
import { helper } from "./helper";
export type { Config } from "./config";
export { helper } from "./helper";
import * as fs from "fs";
`
	results := pack.AnalyzeFileContent("src/a.ts", []byte(src))

	runtime := valuesOf(results.Snippets, file.ComponentImport)
	erased := valuesOf(results.Snippets, file.ComponentImportTypeOnly)

	// The inline `type Inline` sits in a statement that still imports `run`,
	// so the statement is a runtime import and belongs with the others.
	assert.ElementsMatch(t, []string{"./runner", "./helper", "fs", "./helper"}, runtime)
	assert.ElementsMatch(t, []string{"./config", "./thing", "./config"}, erased)
}

func valuesOf(snippets []*file.Snippet, snippetType string) []string {
	var out []string
	for _, s := range snippets {
		if s.Type == snippetType {
			out = append(out, s.Value)
		}
	}
	return out
}
