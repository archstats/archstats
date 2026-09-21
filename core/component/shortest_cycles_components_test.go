package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A Cycle repeats its first element to close the loop, so counting components
// by ranging over it counted the component a cycle starts at twice. That fed
// cycles__short__count, which read 106 for a component in 53 real cycles.
func TestCycleComponents(t *testing.T) {
	t.Run("drops the repeated element that closes the loop", func(t *testing.T) {
		cycle := Cycle{"a", "b", "c", "a"}
		assert.Equal(t, []string{"a", "b", "c"}, cycle.Components())
	})

	t.Run("handles a mutual pair", func(t *testing.T) {
		assert.Equal(t, []string{"a", "b"}, Cycle{"a", "b", "a"}.Components())
	})

	t.Run("counts every component of a cycle exactly once", func(t *testing.T) {
		cycle := Cycle{"a", "b", "c", "a"}
		counts := map[string]int{}
		for _, cmpnt := range cycle.Components() {
			counts[cmpnt]++
		}
		for _, cmpnt := range cycle.Components() {
			assert.Equal(t, 1, counts[cmpnt], "component %s counted %d times", cmpnt, counts[cmpnt])
		}
	})

	t.Run("leaves an already-open path alone", func(t *testing.T) {
		assert.Equal(t, []string{"a", "b"}, Cycle{"a", "b"}.Components())
	})

	t.Run("has nothing to return for an empty cycle", func(t *testing.T) {
		assert.Empty(t, Cycle{}.Components())
	})
}
