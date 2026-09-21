package python

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
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

// django-oscar resolves 746 of its dependencies from strings at runtime --
// roughly 31% of its edges -- because every app must be overridable. Read as
// static imports only, a third of that codebase's graph is missing.
func TestDynamicLookups(t *testing.T) {
	pack := createPythonLanguagePack()
	src := `
from django.db import models
from oscar.core.loading import get_class, get_model

ProductDetailView = get_class("catalogue.views", "ProductDetailView")
Basket = get_model("basket", "Basket")
Repo, Applicator = get_classes("offer.utils", ["Repository", "Applicator"])
mod = importlib.import_module("shipping.methods")
computed = get_class(some_variable, "Thing")
`
	results := pack.AnalyzeFileContent("src/shop/views.py", []byte(src))

	dynamic := pyValues(results.Snippets, file.ComponentImportDynamic)
	// The last call names its module with a variable, so there is no string
	// to capture and nothing to resolve -- which is the honest answer.
	assert.ElementsMatch(t, []string{"catalogue.views", "basket", "offer.utils", "shipping.methods"}, dynamic)

	static := pyValues(results.Snippets, file.ComponentImport)
	assert.Contains(t, static, "django.db")
	assert.Contains(t, static, "oscar.core.loading")
}

func pyValues(snippets []*file.Snippet, snippetType string) []string {
	var out []string
	for _, s := range snippets {
		if s.Type == snippetType {
			out = append(out, s.Value)
		}
	}
	return out
}
