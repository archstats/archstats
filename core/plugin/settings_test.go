package plugin

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var described = []*Descriptor{{
	Name: "example",
	Arguments: Arguments{
		"since":  {Type: String, Default: ""},
		"limit":  {Type: Int, Default: 100},
		"strict": {Type: Bool, Default: true},
		"extra":  {Type: StringSlice, Default: []string{}},
	},
}}

func TestValues_FallsBackToTheDeclaredDefaults(t *testing.T) {
	s := Values(nil, described)

	limit, err := s.GetInt("limit")
	require.NoError(t, err)
	assert.Equal(t, 100, limit)
	strict, err := s.GetBool("strict")
	require.NoError(t, err)
	assert.True(t, strict)
	extra, err := s.GetStringSlice("extra")
	require.NoError(t, err)
	assert.Empty(t, extra)
}

func TestValues_ReadsAMapDecodedFromJSON(t *testing.T) {
	s := Values(map[string]any{"limit": float64(0), "extra": []any{"a", "b"}, "since": "2025-01-01"}, described)

	limit, err := s.GetInt("limit")
	require.NoError(t, err)
	assert.Equal(t, 0, limit)
	extra, err := s.GetStringSlice("extra")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, extra)
	since, err := s.GetString("since")
	require.NoError(t, err)
	assert.Equal(t, "2025-01-01", since)
}

func TestValues_SaysWhatIsWrong(t *testing.T) {
	s := Values(map[string]any{"limit": 1.5, "strict": "yes"}, described)

	_, err := s.GetInt("limit")
	assert.ErrorContains(t, err, "limit")
	_, err = s.GetBool("strict")
	assert.ErrorContains(t, err, "strict")
	_, err = s.GetString("missing")
	assert.ErrorContains(t, err, "missing")
}

// The CLI hands extensions its flag set as it is.
func TestAFlagSetIsSettings(t *testing.T) {
	var _ Settings = pflag.NewFlagSet("x", pflag.ContinueOnError)
}
