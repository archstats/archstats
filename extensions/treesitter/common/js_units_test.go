package common

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
)

func snip(snippetType, value string, offset int) *file.Snippet {
	return &file.Snippet{Type: snippetType, Value: value, Begin: &file.Position{Offset: offset}}
}

func ids(units []*unit.Unit) []string {
	var out []string
	for _, u := range units {
		out = append(out, u.ID)
	}
	return out
}

// The module is the file: there is no package clause, and two files may each
// export a `handler`.
func TestModuleOf(t *testing.T) {
	assert.Equal(t, "client/src/utils/rules", ModuleOf("client/src/utils/rules.ts"))
	assert.Equal(t, "client/src/App", ModuleOf("client/src/App.tsx"))
	assert.Equal(t, "lib/router", ModuleOf("lib/router.js"))
	// The walker writes a root-level file as "./index.ts".
	assert.Equal(t, "index", ModuleOf("./index.ts"))
	// An index file is named by its directory, the way an import writes it:
	// `from "~/components"` means components/index.ts.
	assert.Equal(t, "src/components", ModuleOf("src/components/index.ts"))
	assert.Equal(t, "src/components", ModuleOf("src/components/index.tsx"))
}

// LibreChat is 997 functions to 14 classes. A function has to be a unit here
// or the view is empty.
func TestFunctionsAndClassesAreBothUnits(t *testing.T) {
	res := &file.Results{Snippets: []*file.Snippet{
		snip(CaptureJSClass, "ChatStore", 0),
		snip(CaptureJSFunction, "useChat", 100),
		snip(CaptureJSFunction, "MessageList", 200),
	}}

	units := JSUnitsFrom("client/src/chat.ts", res)
	assert.ElementsMatch(t, []string{
		"client/src/chat#ChatStore",
		"client/src/chat#useChat",
		"client/src/chat#MessageList",
	}, ids(units))

	byID := map[string]*unit.Unit{}
	for _, u := range units {
		byID[u.ID] = u
	}
	assert.Equal(t, unit.KindType, byID["client/src/chat#ChatStore"].Kind)
	assert.Equal(t, unit.KindFunction, byID["client/src/chat#useChat"].Kind)
}

// A method belongs to the class above it.
func TestMethodsBelongToTheClassAboveThem(t *testing.T) {
	res := &file.Results{Snippets: []*file.Snippet{
		snip(CaptureJSClass, "Alpha", 0),
		snip(CaptureJSMethod, "run", 50),
		snip(CaptureJSClass, "Beta", 100),
		snip(CaptureJSMethod, "run", 150),
	}}

	byID := map[string]*unit.Unit{}
	for _, u := range JSUnitsFrom("src/two.ts", res) {
		byID[u.ID] = u
	}

	// Two methods with the same name do not collide, because each is
	// qualified by the class it belongs to.
	assert.Equal(t, "src/two#Alpha", byID["src/two#Alpha.run"].Owner)
	assert.Equal(t, "src/two#Beta", byID["src/two#Beta.run"].Owner)
}

// A method declared before any class belongs to nothing rather than to the
// wrong thing.
func TestMethodBeforeAnyClassHasNoOwner(t *testing.T) {
	res := &file.Results{Snippets: []*file.Snippet{
		snip(CaptureJSMethod, "orphan", 0),
		snip(CaptureJSClass, "Later", 100),
	}}
	byID := map[string]*unit.Unit{}
	for _, u := range JSUnitsFrom("src/a.ts", res) {
		byID[u.ID] = u
	}
	assert.Empty(t, byID["src/a#orphan"].Owner)
}

// A decorator sits above the class it is about, which is the only thing in
// this family that looks like an annotation -- and it exists only in Angular
// and NestJS.
func TestDecoratorsAttachToTheClassBelowThem(t *testing.T) {
	res := &file.Results{Snippets: []*file.Snippet{
		snip(CaptureJSDecorator, "Injectable", 0),
		snip(CaptureJSClass, "UserService", 20),
		snip(CaptureJSClass, "Plain", 200),
	}}

	byID := map[string]*unit.Unit{}
	for _, u := range JSUnitsFrom("src/user.ts", res) {
		byID[u.ID] = u
	}
	assert.True(t, byID["src/user#UserService"].HasMarker("Injectable"))
	assert.False(t, byID["src/user#Plain"].HasMarker("Injectable"))
}

func TestInterfacesAreTypesAndMarkedAsSuch(t *testing.T) {
	res := &file.Results{Snippets: []*file.Snippet{snip(CaptureJSInterface, "Config", 0)}}
	units := JSUnitsFrom("src/config.ts", res)
	assert.Equal(t, unit.KindType, units[0].Kind)
	assert.True(t, units[0].HasMarker("interface"))
}

func TestNothingDeclaredIsNoUnits(t *testing.T) {
	assert.Empty(t, JSUnitsFrom("src/empty.ts", &file.Results{}))
}
