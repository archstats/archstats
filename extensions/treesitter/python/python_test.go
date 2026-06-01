package python

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPythonLanguagePack(t *testing.T) {
	pack := createPythonLanguagePack()

	content := `
import os
from sys import argv
from .db import DB
from ..utils.crypto import Crypto

@app.get("/users")
def get_users():
    pass

@app.route("/items", methods=["POST"])
def post_item():
    pass

@route("/health")
def health():
    pass

class UserAuthService:
    pass
`

	results := pack.AnalyzeFileContent("src/auth.py", []byte(content))
	assert.NotNil(t, results)

	// Check total types (UserAuthService class)
	var classes []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "modularity__types__total" {
			classes = append(classes, snippet.Value)
		}
	}
	assert.Contains(t, classes, "UserAuthService")

	// Check imports (regular, absolute, relative)
	var imports []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "modularity__component__imports" {
			imports = append(imports, snippet.Value)
		}
	}
	assert.Contains(t, imports, "os")
	assert.Contains(t, imports, "sys")
	assert.Contains(t, imports, ".db")
	assert.Contains(t, imports, "..utils.crypto")

	// Check Python web routes (decorators: get, route)
	var routes []string
	for _, snippet := range results.Snippets {
		if snippet.Type == "python__web__routes" {
			routes = append(routes, snippet.Value)
		}
	}
	assert.Contains(t, routes, "get")
	assert.Contains(t, routes, "route")
}
