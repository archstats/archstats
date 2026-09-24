package python

import (
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unitsIn(t *testing.T, path, src string) map[string]*unit.Unit {
	t.Helper()
	res := createPythonLanguagePack().AnalyzeFileContent(path, []byte(src))
	require.NotNilf(t, res, "%s was not analysed", path)
	out := map[string]*unit.Unit{}
	for _, u := range unitsFrom(path, []byte(src), res) {
		out[u.ID] = u
	}
	return out
}

// In Django the role is in the filename. django-oscar has 29 apps.py, 25
// views.py and 24 models.py, against 39 architectural decorators in the
// whole repository: nothing in the class says what it is.
func TestFilenameIsEvidence(t *testing.T) {
	byID := unitsIn(t, "src/shop/catalogue/models.py", "class Product:\n    pass\n")

	product := byID["src/shop/catalogue/models#Product"]
	require.NotNil(t, product)
	assert.True(t, product.HasMarker("models"))

	// And the evidence says what kind it is, because "filename:models" is a
	// different claim from "annotation:models".
	var source string
	for _, m := range product.Markers {
		if m.Key == "models" {
			source = m.Source
		}
	}
	assert.Equal(t, unit.SourceFilename, source)
}

// A file Django gives no meaning marks nothing; a marker on every file would
// be noise rather than evidence.
func TestOrdinaryFilenamesMarkNothing(t *testing.T) {
	byID := unitsIn(t, "src/shop/helpers.py", "class Thing:\n    pass\n")
	assert.Empty(t, byID["src/shop/helpers#Thing"].Markers)
}

// Python nests a method inside its class, so indentation is what separates a
// method from a module-level function that merely follows the class.
func TestMethodsBelongToTheirClassAndModuleFunctionsDoNot(t *testing.T) {
	src := `class Basket:
    def add(self):
        pass

def helper():
    pass
`
	byID := unitsIn(t, "src/shop/basket/models.py", src)

	add := byID["src/shop/basket/models#Basket.add"]
	require.NotNil(t, add, "a method is qualified by its class")
	assert.Equal(t, "src/shop/basket/models#Basket", add.Owner)
	assert.Equal(t, unit.KindFunction, add.Kind)

	helper := byID["src/shop/basket/models#helper"]
	require.NotNil(t, helper)
	assert.Empty(t, helper.Owner, "a module-level function follows the class, it is not in it")
}

// The decorators Python does use -- FastAPI routes, Django receivers --
// belong to the thing directly below them.
func TestDecoratorsAttachToWhatFollows(t *testing.T) {
	src := `@receiver
def on_save():
    pass

def plain():
    pass
`
	byID := unitsIn(t, "src/shop/signals.py", src)
	assert.True(t, byID["src/shop/signals#on_save"].HasMarker("receiver"))
	assert.False(t, byID["src/shop/signals#plain"].HasMarker("receiver"))
}

func TestClassesAndFunctionsAreBothUnits(t *testing.T) {
	byID := unitsIn(t, "src/a/views.py", "class View:\n    pass\n\ndef render():\n    pass\n")
	assert.Equal(t, unit.KindType, byID["src/a/views#View"].Kind)
	assert.Equal(t, unit.KindFunction, byID["src/a/views#render"].Kind)
}

func TestRelativeImportsBecomePaths(t *testing.T) {
	cases := map[string]string{
		".models":      "./models",
		"..core.utils": "../core/utils",
		"...x":         "../../x",
		".":            ".",
		"oscar.core":   "oscar.core",
	}
	for in, want := range cases {
		if got := relativeToPath(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

// A Django model declares its Meta, then its methods. The methods belonged
// to the last class above them, which is Meta, and were indented left of it,
// so they belonged to nothing: oscar's `__str__` and `clean` were
// module-level functions, one per name per file.
func TestMethodsAfterANestedClassBelongToTheOuterOne(t *testing.T) {
	src := `from django.db import models


class Address(models.Model):
    line1 = models.CharField()

    class Meta:
        abstract = True

    def __str__(self):
        return self.line1

    def clean(self):
        pass


class Country(models.Model):
    class Meta:
        ordering = ["name"]

    def __str__(self):
        return "x"


def module_level():
    return 1
`
	byID := unitsIn(t, "oscar/apps/address/abstract_models.py", src)
	meta := byID["oscar/apps/address/abstract_models#Address.Meta"]
	require.NotNil(t, meta, "a nested class is named through its class")
	assert.Equal(t, "oscar/apps/address/abstract_models#Address", meta.Owner)
	require.Contains(t, byID, "oscar/apps/address/abstract_models#Country.Meta")

	str := byID["oscar/apps/address/abstract_models#Address.__str__"]
	require.NotNil(t, str)
	assert.Equal(t, "oscar/apps/address/abstract_models#Address", str.Owner)
	require.Contains(t, byID, "oscar/apps/address/abstract_models#Address.clean")
	require.Contains(t, byID, "oscar/apps/address/abstract_models#Country.__str__")
	assert.NotContains(t, byID, "oscar/apps/address/abstract_models#__str__")
	assert.Empty(t, byID["oscar/apps/address/abstract_models#module_level"].Owner)
}
