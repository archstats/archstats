#!/usr/bin/env bash
# Writes the fixture snapshot that e2eTest/testdata/snapshot_schema.golden
# describes, freshly scanned by this checkout, so archstats-ui's tests can
# query a real snapshot instead of a hand-made one:
#
#   scripts/ui-fixture-db.sh                 # -> ./ui-fixture.db
#   scripts/ui-fixture-db.sh ../archstats-ui/testdata/ui-fixture.db
#
# The scan is deterministic (fixed commit dates, scanned_at at the epoch),
# so the same engine writes the same rows. The test still fails if the
# schema moved from the golden; the file is written either way.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:-ui-fixture.db}"
case "$out" in
  /*) ;;
  *) out="$PWD/$out" ;;
esac

cd "$root"
go test -count=1 ./e2eTest -run 'TestSnapshotSchema$' -export-db="$out"
echo "wrote $out"
