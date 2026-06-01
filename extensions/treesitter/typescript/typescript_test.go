package typescript

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
