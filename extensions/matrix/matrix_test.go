package matrix

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"UserController", []string{"user", "controller"}},
		{"user_controller", []string{"user", "controller"}},
		{"user-controller", []string{"user", "controller"}},
		{"User2", []string{"user", "2"}},
		{"simple", []string{"simple"}},
		{"XMLParser", []string{"xml", "parser"}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			actual := tokenize(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestGetCoreTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"UserController", []string{"user"}},
		{"UserRepository", []string{"user"}},
		{"UserServiceImpl", []string{"user"}},
		{"BillingService", []string{"billing"}},
		{"ServiceUtil", []string{}}, // both filtered out
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			actual := getCoreTokens(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestJaccardSimilarity(t *testing.T) {
	assert.InDelta(t, 1.0, jaccardSimilarity([]string{"user"}, []string{"user"}), 1e-9)
	assert.InDelta(t, 0.5, jaccardSimilarity([]string{"user", "profile"}, []string{"user"}), 1e-9)
	assert.InDelta(t, 0.0, jaccardSimilarity([]string{"user"}, []string{"product"}), 1e-9)
	assert.InDelta(t, 0.0, jaccardSimilarity([]string{}, []string{"user"}), 1e-9)
}

func TestComputePathDistances(t *testing.T) {
	nodes := []string{"A", "B", "C", "D"}
	adj := map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"D"},
	}

	distances := computePathDistances(nodes, adj)

	// A's distances
	assert.Equal(t, 0, distances["A"]["A"])
	assert.Equal(t, 1, distances["A"]["B"])
	assert.Equal(t, 2, distances["A"]["C"])
	assert.Equal(t, 3, distances["A"]["D"])

	// B's distances
	assert.Equal(t, 0, distances["B"]["B"])
	assert.Equal(t, 1, distances["B"]["C"])
	assert.Equal(t, 2, distances["B"]["D"])
	_, exists := distances["B"]["A"]
	assert.False(t, exists) // A is unreachable from B
}
