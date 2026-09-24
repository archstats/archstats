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
const AnalysisRevision = 1
