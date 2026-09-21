package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Folding is the whole reason units exist: without it a C# partial class is
// as many types as it has files, and nopCommerce declares 1,567 of them.
func TestMerge_FoldsOneTypeAcrossFiles(t *testing.T) {
	merged := Merge([]*Unit{
		{ID: "Acme.Customer", Kind: KindType, Name: "Customer", Files: []string{"b.cs"},
			Markers: []Marker{{Source: SourceAnnotation, Key: "Table"}}},
		{ID: "Acme.Customer", Kind: KindType, Name: "Customer", Files: []string{"a.cs"},
			Markers: []Marker{{Source: SourceSupertype, Key: "BaseEntity"}}},
	})

	require.Len(t, merged, 1)
	// Sorted, so an export diffs cleanly between runs.
	assert.Equal(t, []string{"a.cs", "b.cs"}, merged[0].Files)
	// Neither half's evidence is lost. A partial carrying the annotations and
	// one carrying none must not cancel out.
	assert.True(t, merged[0].HasMarker("Table"))
	assert.True(t, merged[0].HasMarker("BaseEntity"))
}

func TestMerge_KeepsDistinctUnitsApart(t *testing.T) {
	merged := Merge([]*Unit{
		{ID: "Acme.B", Kind: KindType, Files: []string{"x.cs"}},
		{ID: "Acme.A", Kind: KindType, Files: []string{"x.cs"}},
	})
	require.Len(t, merged, 2)
	assert.Equal(t, "Acme.A", merged[0].ID, "ordered by id, so views are stable")
	assert.Equal(t, "Acme.B", merged[1].ID)
}

// A unit with no id is not a unit. Dropping it beats inventing an entity
// keyed on the empty string that everything else then folds into.
func TestMerge_DropsUnidentifiable(t *testing.T) {
	assert.Empty(t, Merge([]*Unit{nil, {Kind: KindType, Name: "anonymous"}}))
}

func TestMerge_DoesNotMutateInput(t *testing.T) {
	original := &Unit{ID: "Acme.A", Kind: KindType, Files: []string{"a.cs"}}
	Merge([]*Unit{original, {ID: "Acme.A", Kind: KindType, Files: []string{"b.cs"}}})
	assert.Equal(t, []string{"a.cs"}, original.Files, "callers keep their own slices")
}

func TestMerge_DeduplicatesRepeatedEvidence(t *testing.T) {
	merged := Merge([]*Unit{
		{ID: "A", Kind: KindType, Files: []string{"a.go"}, Markers: []Marker{{Source: SourceAnnotation, Key: "Service"}}},
		{ID: "A", Kind: KindType, Files: []string{"a.go"}, Markers: []Marker{{Source: SourceAnnotation, Key: "Service"}}},
	})
	require.Len(t, merged, 1)
	assert.Len(t, merged[0].Files, 1)
	assert.Len(t, merged[0].Markers, 1)
}

// The same word from two kinds of evidence is two different claims.
// "filename:models" and "annotation:models" must not collapse into one.
func TestMarkers_SourceDistinguishesEvidence(t *testing.T) {
	u := Merge([]*Unit{{ID: "A", Kind: KindType, Files: []string{"a.py"}, Markers: []Marker{
		{Source: SourceFilename, Key: "models"},
		{Source: SourceAnnotation, Key: "models"},
	}}})[0]

	assert.Len(t, u.Markers, 2)
	assert.True(t, u.HasMarker("models"))
}

func TestMarkerValues(t *testing.T) {
	u := &Unit{Markers: []Marker{
		{Source: SourceStructTag, Key: "json", Value: "id"},
		{Source: SourceStructTag, Key: "json", Value: "name"},
		{Source: SourceStructTag, Key: "form", Value: "id"},
	}}
	assert.Equal(t, []string{"id", "name"}, u.MarkerValues("json"))
	assert.Empty(t, u.MarkerValues("absent"))
}
