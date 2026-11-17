#!/usr/bin/env bash
# scan-parity.sh: does the working tree scan the same as an older commit?
#
#   scripts/scan-parity.sh [-i table.column,...] OLD_REF [DIR...]
#
# Builds archstats at OLD_REF (in a throwaway worktree) and from the working
# tree, exports a snapshot of each DIR with both, and compares them table by
# table. DIRs default to every checkout in e2eTest/temp_testdata. Exits 1 if
# any snapshot differs, printing the rows that moved.
#
# A refactor must pass this untouched. A change that is meant to alter scans
# bumps core.AnalysisRevision instead (see core/revision.go), and this shows
# what it altered. -i leaves columns out of the comparison, for example
# -i '*.page_rank' when an old binary still predates deterministic scans.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
ignore=""
if [[ "${1:-}" == "-i" ]]; then
  ignore="$2"
  shift 2
fi
old_ref="${1:?usage: scan-parity.sh [-i table.column,...] OLD_REF [DIR...]}"
shift

if [[ $# -eq 0 ]]; then
  set -- "$root"/e2eTest/temp_testdata/*/
fi

work="$(mktemp -d)"
trap 'git -C "$root" worktree remove --force "$work/old-src" >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

echo "building $old_ref and the working tree"
git -C "$root" worktree add --detach -q "$work/old-src" "$old_ref"
# The Swift grammar's generated C is gitignored; reuse this checkout's copy.
cp "$root"/extensions/treesitter/swift/grammar/*.c "$work/old-src/extensions/treesitter/swift/grammar/" 2>/dev/null || true
cp -R "$root"/extensions/treesitter/swift/grammar/tree_sitter "$work/old-src/extensions/treesitter/swift/grammar/" 2>/dev/null || true
(cd "$work/old-src" && go build -o "$work/old" .)
(cd "$root" && go build -o "$work/new" . && go build -o "$work/snapdiff" ./internal/snapdigest/snapdiff)

status=0
for dir in "$@"; do
  dir="${dir%/}"
  name="$(basename "$dir")"
  "$work/old" export sqlite "$work/$name.old.db" -f "$dir" --store-content=false >/dev/null 2>&1
  "$work/new" export sqlite "$work/$name.new.db" -f "$dir" --store-content=false >/dev/null 2>&1
  if out="$("$work/snapdiff" -ignore "$ignore" "$work/$name.old.db" "$work/$name.new.db")"; then
    echo "same      $name"
  else
    echo "DIFFERENT $name"
    printf '%s\n' "$out" | sed 's/^/    /'
    status=1
  fi
done
exit $status
