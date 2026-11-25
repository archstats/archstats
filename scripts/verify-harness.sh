#!/usr/bin/env bash
# The definition of done for a change to this repository (AGENTS.md). Runs,
# in order, stopping at the first failure:
#
#   1. the tests: the fast suite (go test -short ./...), which holds the
#      snapshot schema golden archstats-ui depends on, the scan digest, the
#      import boundaries and the fuzz seeds. --full drops -short and adds the
#      real projects and the scan budget, as CI's thorough job does;
#   2. golangci-lint (.golangci.yml): the depguard layer rules, forbidigo,
#      and what the general linters report over main;
#   3. Archstats on itself: archstats.rules.yaml, which includes no import
#      cycle among core and extensions, with the ignores CI's self-check uses.
#
#   scripts/verify-harness.sh [--full]
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

short=(-short)
case "${1:-}" in
  --full) short=() ;;
  "") ;;
  *) echo "usage: $0 [--full]" >&2; exit 2 ;;
esac

step() { printf '\n==> %s\n' "$*"; }

if ! ls extensions/treesitter/swift/grammar/*.c >/dev/null 2>&1; then
  step "fetching the Swift grammar (once per clone)"
  go generate ./extensions/treesitter/swift/grammar
fi

step "go test ${short[*]:-} ./..."
go test "${short[@]}" ./...

step "golangci-lint run ./..."
if ! command -v golangci-lint >/dev/null; then
  echo "golangci-lint is not installed; CI runs v2.15. Install it: https://golangci-lint.run/welcome/install/" >&2
  exit 1
fi
golangci-lint run ./...

step "archstats assert -r archstats.rules.yaml (Archstats on itself)"
go run . assert -r archstats.rules.yaml -f . \
  --ignore 'e2eTest/**' --ignore 'tasks/**' --ignore 'docs/**' --ignore 'scratch/**' --ignore '.claude/**'

step "verify-harness: all checks passed"
