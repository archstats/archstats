package sqlite

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestIncludedExcluded(t *testing.T) {

}

func TestViewInclusion(t *testing.T) {
	tests := []struct {
		name     string
		possible string
		excluded string
		included string

		shouldError  bool
		shouldReturn []string
	}{

		{"no flags", "a,b,c", "", "", false, []string{"a", "b", "c"}},

		{"only excluded", "a,b,c", "b", "", false, []string{"a", "c"}},
		{"only excluded multiple", "a,b,c", "b,c", "", false, []string{"a"}},

		{"only included", "a,b,c", "", "b", false, []string{"b"}},
		{"only included multiple", "a,b,c", "", "b,c", false, []string{"b", "c"}},

		{"included and excluded", "a,b,c", "b", "c", false, []string{"c"}},

		{"invalid included", "a,b,c", "", "d", true, nil},
		{"invalid excluded", "a,b,c", "d", "", true, nil},
		{"invalid included and excluded", "a,b,c", "d", "e", true, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			possibleSlice := strings.Split(test.possible, ",")
			excludedSlice := strings.Split(test.excluded, ",")
			includedSlice := strings.Split(test.included, ",")

			show, err := getViewsToShow(includedSlice, excludedSlice, possibleSlice)

			if test.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, test.shouldReturn, show)
			}

		})
	}
}
