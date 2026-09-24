package common

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
)

func nestSnip(value string, begin, end int) *file.Snippet {
	return &file.Snippet{Value: value, Begin: &file.Position{Offset: begin}, End: &file.Position{Offset: end}}
}

func TestNestedTypesAreNamedThroughTheirEnclosingTypes(t *testing.T) {
	// class Outer { class Middle { class Inner } enum Kind } class Other
	outer, middle, inner, kind, other := nestSnip("Outer", 6, 11), nestSnip("Middle", 20, 26), nestSnip("Inner", 35, 40), nestSnip("Kind", 50, 54), nestSnip("Other", 70, 75)
	spans := []*file.Snippet{nestSnip("", 0, 60), nestSnip("", 14, 45), nestSnip("", 29, 42), nestSnip("", 64, 80)}
	names := NestedNames([]*file.Snippet{outer, middle, inner, kind, other}, spans)
	assert.Equal(t, "Outer", names[outer])
	assert.Equal(t, "Outer.Middle", names[middle])
	assert.Equal(t, "Outer.Middle.Inner", names[inner])
	// An enum has no span of its own in the C# pack; it is still inside Outer.
	assert.Equal(t, "Outer.Kind", names[kind])
	assert.Equal(t, "Other", names[other])
}
