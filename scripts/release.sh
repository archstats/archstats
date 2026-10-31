#!/usr/bin/env bash
# release.sh vX.Y.Z [--push]: cut an engine release.
#
# Checks that main is clean, pushed and green in CI, moves the Unreleased
# notes in CHANGELOG.md under the new version with the analysis revision and
# the commit list, commits that, and makes a signed tag. With --push it pushes
# main and the tag; the release workflow then builds every platform and opens
# a draft release whose notes are that CHANGELOG section. Publish the draft by
# hand once the binaries are checked.
#
# The desktop app pins the engine by tag, so release the engine first and then
# run archstats-ui's scripts/release.sh with --engine.
set -euo pipefail

repo="archstats/archstats"
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

fail() { echo "release: $*" >&2; exit 1; }

version="${1:-}"
push=false
[[ "${2:-}" == "--push" ]] && push=true
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || fail "usage: scripts/release.sh vX.Y.Z [--push]"

[[ "$(git rev-parse --abbrev-ref HEAD)" == "main" ]] || fail "not on main"
git diff --quiet && git diff --cached --quiet || fail "main has uncommitted changes to tracked files"
git fetch -q origin main --tags
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || fail "main is not the same as origin/main: push or pull first"
git rev-parse -q --verify "refs/tags/$version" >/dev/null && fail "$version is already tagged"

previous="$(git describe --tags --abbrev=0 --match 'v*' HEAD)"
[[ "$(printf '%s\n%s\n' "$previous" "$version" | sort -V | tail -1)" == "$version" ]] || fail "$version does not come after $previous"

sha="$(git rev-parse HEAD)"
ci="$(gh run list --repo "$repo" --workflow ci.yml --commit "$sha" --limit 1 --json status,conclusion --jq '.[0] | "\(.status) \(.conclusion)"')"
case "$ci" in
  "completed success") ;;
  "") fail "CI has not run on ${sha:0:7}" ;;
  "completed "*) fail "CI on ${sha:0:7} ended ${ci#completed }" ;;
  *) fail "CI on ${sha:0:7} is still running" ;;
esac

grep -q '^## Unreleased' CHANGELOG.md || fail "CHANGELOG.md has no Unreleased section"
highlights="$(awk '/^## Unreleased/ { f = 1; next } /^## / { f = 0 } f' CHANGELOG.md | sed -e '/./,$!d')"
r0="$(scripts/changelog.sh revision "$previous")"
r1="$(scripts/changelog.sh revision HEAD)"
commits="$(scripts/changelog.sh commits "$previous" HEAD)"

section="## $version ($(date +%F))"$'\n\n'
[[ -n "$highlights" ]] && section+="$highlights"$'\n\n'
[[ "$r0" != "$r1" ]] && section+="Analysis revision $r1 (was $r0): snapshots taken before this release are flagged for a rescan."$'\n\n'
section+="${commits:-Maintenance only.}"

{
  awk '/^## Unreleased/ { exit } { print }' CHANGELOG.md
  printf '## Unreleased\n\n%s\n\n' "$section"
  awk '/^## Unreleased/ { f = 1; next } f && /^## / { p = 1 } p' CHANGELOG.md
} | cat -s > CHANGELOG.md.new
mv CHANGELOG.md.new CHANGELOG.md

echo "------ $version"
scripts/changelog.sh section "$version"
echo "------"

git add CHANGELOG.md
git commit -q -m "chore(release): $version"
git tag -s "$version" -m "$version"
echo "Tagged $version on $(git rev-parse --short HEAD)."

if $push; then
  git push origin main "refs/tags/$version"
  echo "Pushed. The draft release appears at https://github.com/$repo/releases once the workflow finishes."
else
  echo "Push with: git push origin main refs/tags/$version"
fi
echo "Then pin the desktop app to it: (in archstats-ui) scripts/release.sh vA.B.C --engine $version"
