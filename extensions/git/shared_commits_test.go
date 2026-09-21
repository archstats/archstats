package git

import (
	"testing"

	"github.com/archstats/archstats/extensions/git/commits"
	"github.com/stretchr/testify/assert"
)

// A component can carry shared commits while its own commit count is zero —
// a shallow clone, or history the filters took out. Dividing by that zero
// gives +Inf, which is not NaN, so the guard that was there never fired:
// nearly 50,000 of nopCommerce's 82,558 co-change rows shipped "+Inf" as a
// percentage.
func TestSharedCommits_PercentageOfNoCommitsIsZero(t *testing.T) {
	row := toRow(
		"a", "b",
		commits.CommitHashes{"c1"},
		map[int]commits.CommitHashes{},
		map[string]commits.CommitHashes{
			// "a" has no commits of its own at all.
			"b": {"c1", "c2"},
		},
		map[int]map[string]commits.CommitHashes{},
	)

	assert.Equal(t, 0.0, row.Data[PercentageOfAllCommitsPair1], "a percentage of no commits must be zero, not an infinity")
	assert.Equal(t, 50.0, row.Data[PercentageOfAllCommitsPair2])
}

// Every __LAST_N_DAYS shared-commit column came out zero on every repository,
// however recent the work, because the two branches picking the bucket's
// commits were the wrong way round.
func TestSharedCommits_DayBucketsCarryTheirCommits(t *testing.T) {
	row := toRow(
		"a", "b",
		commits.CommitHashes{"c1", "c2"},
		map[int]commits.CommitHashes{30: {"c1"}},
		map[string]commits.CommitHashes{"a": {"c1", "c2"}, "b": {"c1", "c2"}},
		map[int]map[string]commits.CommitHashes{30: {"a": {"c1", "c2"}, "b": {"c1"}}},
	)

	assert.Equal(t, 1, row.Data[toDayStat(SharedCommitCount, 30)], "the 30-day bucket dropped its shared commits")
	assert.Equal(t, 50.0, row.Data[toDayStat(PercentageOfAllCommitsPair1, 30)])
	assert.Equal(t, 100.0, row.Data[toDayStat(PercentageOfAllCommitsPair2, 30)])
}
