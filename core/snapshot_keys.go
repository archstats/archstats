package core

// KnownSnapshotKeys is every key a scan may write to the _snapshot table.
// DESCRIPTION.md documents each one, and a test holds it to that.
var KnownSnapshotKeys = []string{
	"analysis_revision",
	"scanned_at",
	"report_id",
	"git_head_commit",
	"git_branch",
	"git_head_time",
	"git_dirty_files",
	"git_based_on",
	"git_max_changes_per_commit",
	"git_sweeping_commits",
	"extensions",
	"walker_ignored_files",
	"walker_ignored_dirs",
	"walker_ignored_top",
}
