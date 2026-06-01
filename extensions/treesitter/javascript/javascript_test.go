package javascript

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestJavaScriptLanguagePack(t *testing.T) {
	pack := createJavaScriptLanguagePack()

	content := `
import { useState, useEffect } from 'react';
import { Title } from './Title';
import layout from '../layout/layout';

class ButtonHelper {}

function ButtonComponent() {
  const [state, setState] = useState(null);
  useEffect(() => {}, []);
  return null;
}

const ArrowComponent = () => {
  const val = useQuery();
  return null;
}
`

	results := pack.AnalyzeFileContent("src/Button.js", []byte(content))
	assert.NotNil(t, results)

	// Check total types (ButtonHelper class)
	var classes []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "modularity__types__total" {
			classes = append(classes, snippet.Value)
		}
	}
	assert.Contains(t, classes, "ButtonHelper")

	// Check imports (should strip quotes)
	var imports []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "modularity__component__imports" {
			imports = append(imports, snippet.Value)
		}
	}
	assert.Contains(t, imports, "react")
	assert.Contains(t, imports, "./Title")
	assert.Contains(t, imports, "../layout/layout")

	// Check React components
	var reactComponents []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "js__react__components" {
			reactComponents = append(reactComponents, snippet.Value)
		}
	}
	assert.Contains(t, reactComponents, "ButtonComponent")
	assert.Contains(t, reactComponents, "ArrowComponent")

	// Check React Hooks
	var reactHooks []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "js__react__hooks" {
			reactHooks = append(reactHooks, snippet.Value)
		}
	}
	assert.Contains(t, reactHooks, "useState")
	assert.Contains(t, reactHooks, "useEffect")
	assert.Contains(t, reactHooks, "useQuery")
}
