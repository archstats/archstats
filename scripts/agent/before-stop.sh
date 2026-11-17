#!/usr/bin/env bash
# Claude Code Stop hook, wired in .claude/settings.json. Before an agent
# says it is done with a change to Go code, run the checks agents most often
# skip, and send it back to work when one fails:
#   - the import and ecosystem boundaries (ADR 0025);
#   - the snapshot schema and scan digest goldens, which also hold
#     core.AnalysisRevision to the scans (e2eTest/scan_digest_test.go);
#   - the same tree scanning the same, and the fuzz seeds;
#   - Archstats' rules for its own code (archstats.rules.yaml) need a scan
#     of the repository and run in CI, not here.
# A tree that already passed is not checked again: the hook remembers the
# hash of the diff it last passed, in the git directory.
set -uo pipefail

command -v jq >/dev/null || exit 0
input="$(cat)"
root="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
cd "$root"

changed="$( { git diff --name-only HEAD; git ls-files --others --exclude-standard; } 2>/dev/null \
  | grep -E '\.go$|\.golden$|^core/revision\.go$|^archstats\.rules\.yaml$' | grep -v '/temp_testdata/' )"
[[ -n "$changed" ]] || exit 0

state="$(git rev-parse --git-dir)/archstats-stop-hook.ok"
hash="$( { git diff HEAD; git ls-files --others --exclude-standard -z | xargs -0 cat 2>/dev/null; } | shasum | cut -c1-40)"
[[ -f "$state" && "$(cat "$state")" == "$hash" ]] && exit 0

out="$(
  go test -short -count=1 ./engine -run 'TestImportBoundaries|TestCoreNamesNoEcosystem|FuzzScan' 2>&1 &&
  go test -short -count=1 ./e2eTest -run 'TestSnapshotSchema|TestScanDigest|TestSnapshot_SameTreeSameSnapshot|TestDescriptionNamesEveryTable' 2>&1
)"
if [[ $? -eq 0 ]]; then
  echo "$hash" > "$state"
  exit 0
fi

# Asked once already and still failing: let the agent stop and report it,
# rather than loop.
if [[ "$(jq -r '.stop_hook_active // false' <<<"$input")" == "true" ]]; then
  exit 0
fi
reason="$(printf '%s\n' "$out" | grep -v '^{' | grep -E -- '--- FAIL|_test\.go:[0-9]+|FAIL|Error' | head -30)"
jq -n --arg r "The pre-finish checks failed. Fix them, or say plainly which still fail and why:
$reason" '{decision: "block", reason: $r}'
exit 0
