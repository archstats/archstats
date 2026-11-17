#!/usr/bin/env bash
# check-changelog.sh: does a change a user would notice say so in CHANGELOG.md?
#
#   scripts/check-changelog.sh BASE_REF
#
# Compares the working tree's HEAD with BASE_REF (in CI, the pull request's
# base branch). Fails when:
#   - non-test Go code that ships (cmd/, core/, engine/, extensions/, main.go)
#     changed but CHANGELOG.md did not. A refactor, a test or a tool change
#     that nobody would notice can skip this with the pull request label
#     `skip-changelog`, which CI checks before running this script;
#   - core.AnalysisRevision moved but no line under Unreleased names the new
#     revision as "revision N". The release notes say which scans need a
#     rescan, and they read it from those lines.
# Bash 3.2 compatible.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
base="${1:?usage: check-changelog.sh BASE_REF}"
cd "$root"

merge_base="$(git merge-base "$base" HEAD)"
changed="$(git diff --name-only "$merge_base" HEAD)"

ships="$(printf '%s\n' "$changed" | grep -E '^(cmd|core|engine|extensions)/.*\.go$|^main\.go$' | grep -v '_test\.go$' | grep -v '/testdata/' || true)"
if [[ -n "$ships" ]] && ! printf '%s\n' "$changed" | grep -qx 'CHANGELOG.md'; then
  echo "Code that ships changed, but CHANGELOG.md did not:"
  printf '%s\n' "$ships" | sed 's/^/  /'
  echo
  echo "Add a line under **Unreleased** saying what a user can now do or what reads differently."
  echo "If nobody would notice (a refactor, a test, a tool), label the pull request skip-changelog."
  exit 1
fi

old_rev="$(scripts/changelog.sh revision "$merge_base")"
new_rev="$(scripts/changelog.sh revision HEAD)"
if [[ "$old_rev" != "$new_rev" ]]; then
  unreleased="$(awk '/^## Unreleased/{on=1; next} /^## /{on=0} on' CHANGELOG.md)"
  if ! printf '%s\n' "$unreleased" | grep -q "revision $new_rev"; then
    echo "core.AnalysisRevision moved from $old_rev to $new_rev, but no line under Unreleased in CHANGELOG.md says \"revision $new_rev\"."
    echo "Name it on the line that says what reads differently, so the release notes can say which snapshots need a rescan."
    exit 1
  fi
fi
echo "CHANGELOG.md: ok"
