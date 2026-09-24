package kotlin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What the parser dropped comes back with a name and a span, nested ones
// inside their outer span; a declaration in a comment or a string does not.
func TestLostDeclarationsAreRecoveredFromTheText(t *testing.T) {
	src := `package org.acme

/** A class ULongColumnType in a comment is not a declaration. */
@Suppress("x") class ULongColumnType : ColumnType<ULong>() {
    val s = "class NotAType {"
    inner class Holder(val v: Int)
}

private object Registry

enum class Kind { A, B }

fun interface Hasher {
    fun hash(): Int
}
`
	names, spans := recoverDeclarations([]byte(src), nil)
	var got []string
	for _, n := range names {
		got = append(got, n.Value)
	}
	assert.Equal(t, []string{"ULongColumnType", "Holder", "Registry", "Kind", "Hasher"}, got)
	require.Len(t, spans, 5)
	// The outer body closes after Holder, so Holder is inside it.
	assert.Less(t, spans[0].Begin.Offset, spans[1].Begin.Offset)
	assert.Greater(t, spans[0].End.Offset, spans[1].End.Offset)
	// A body-less declaration ends at its line.
	assert.Equal(t, byte('\n'), src[spans[2].End.Offset])
}

// Nothing is recovered from a file the parser read whole.
func TestNothingIsRecoveredWhenNothingWasLost(t *testing.T) {
	res := createKotlinLanguagePack().AnalyzeFileContent("src/db/Database.kt", []byte(sample))
	require.NotNil(t, res)
	names, _ := recoverDeclarations([]byte(sample), res.Snippets)
	assert.Empty(t, names)
}
