package core

// AnalysisRevision numbers what a scan means, not what the program is called.
//
// A snapshot is immutable, so a fix to the analysis never reaches the scans
// taken before it: an older snapshot keeps reporting the Go internal/ rule as
// kept on a Java codebase, or 22 modules in a PHP one, long after the engine
// stopped saying so. Readers compare the revision a snapshot was written with
// against this one and say when the snapshot predates what the engine knows.
//
// Bump it whenever a change alters what a scan produces for the same source:
// a new or corrected edge, count, rule verdict or classification. Snapshots
// written before the revision existed read as 0.
//
//	1  2026-09-23  rules scoped by ecosystem (applies_when kind); PHP read by
//	               tree-sitter; git identities merged; Go import resolution;
//	               cycles counted once per component; third-party, generated
//	               and non-code files (translations, stylesheets) get no
//	               health reading; shortest_path_length counts hops, not
//	               nodes; commits touching more than 100 files stay in
//	               git_commits but are left out of co-change.
//	2  2026-09-24  history follows moved files; time windows count back from
//	               the scanned commit; co-change no longer credits a component
//	               with commits it never had; snapshots record the commit,
//	               branch and uncommitted files they read, and what the walker
//	               left out; files carry a role; health keeps its deductions;
//	               days since last change; metric categories.
//	3  2026-09-24  smaller snapshots (file_matrix keeps co-changed pairs,
//	               directory co-change from two shared commits, Java class
//	               reachability opt-in); scan-level ignore patterns apply to
//	               files and their history; file-grain shared commits; an
//	               ambiguous dynamic lookup resolves the same way every scan.
//	4  2026-09-25  smaller snapshots again: report_id and timestamp stored as
//	               column defaults; git_commits a view over git_commit_info
//	               and git_commit_files; file_matrix opt-in (--file-matrix);
//	               component_matrix one row per co-changed pair;
//	               component_connections_indirect records next_hop, not the
//	               path text; Java counts no longer duplicated as snippets.
//	5  2026-09-27  files carry a system_kind (build, lockfile, ci, container,
//	               deploy, infra, config); file co-change counts those files
//	               too; deployables: what the workspace builds and ships, what
//	               goes into each, how they are built, where they run, what
//	               they talk to and what technology they carry.
const AnalysisRevision = 5
