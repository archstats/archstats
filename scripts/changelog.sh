#!/usr/bin/env bash
# changelog.sh: the release notes, read from conventional commits and from
# CHANGELOG.md. Used by scripts/release.sh and by the release workflow.
#
#   changelog.sh commits FROM TO     commit list between two refs, grouped by kind
#   changelog.sh section VERSION     a release's section of CHANGELOG.md, without
#                                    its heading: the body of the GitHub release
#   changelog.sh revision REF        core.AnalysisRevision at REF (0 before it existed)
#
# Commit kinds that change nothing a user sees (chore, ci, test, style, build)
# and merges are left out. Bash 3.2 compatible: it runs on a stock Mac.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
changelog="$root/CHANGELOG.md"

# One "kind<TAB>line" per commit; kinds the notes leave out print nothing.
classify() {
  local subject kind scope text
  while IFS= read -r subject; do
    if [[ "$subject" =~ ^([a-z]+)(\(([^\)]*)\))?!?:\ (.*)$ ]]; then
      kind="${BASH_REMATCH[1]}"
      scope="${BASH_REMATCH[3]}"
      text="${BASH_REMATCH[4]}"
    else
      kind="other"
      scope=""
      text="$subject"
    fi
    case "$kind" in
      chore|ci|test|style|build) continue ;;
    esac
    [[ "$subject" == Merge\ * ]] && continue
    [[ -n "$scope" ]] && text="$scope: $text"
    printf '%s\t- %s\n' "$kind" "$text"
  done
}

commits() {
  local from="$1" to="$2" lines group title
  lines="$(git -C "$root" log --no-merges --reverse --format='%s' "$from..$to" | classify)"
  for group in "feat:Features" "fix:Fixes" "perf:Performance" "refactor docs revert other:Other changes"; do
    title="${group#*:}"
    local body=""
    for kind in ${group%%:*}; do
      body+="$(printf '%s\n' "$lines" | awk -F'\t' -v k="$kind" '$1 == k { print $2 }')"$'\n'
    done
    body="$(printf '%s' "$body" | sed '/^$/d')"
    [[ -z "$body" ]] && continue
    printf '### %s\n\n%s\n\n' "$title" "$body"
  done
}

section() {
  local version="$1"
  awk -v v="$version" '
    /^## / { if (inside) exit; inside = ($2 == v) ; next }
    inside { print }
  ' "$changelog" | sed -e '/./,$!d' | sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}'
}

revision() {
  git -C "$root" show "$1:core/revision.go" 2>/dev/null \
    | sed -n 's/^const AnalysisRevision = \([0-9][0-9]*\).*/\1/p' | head -1 | grep . || echo 0
}

case "${1:-}" in
  commits)  commits "$2" "$3" ;;
  section)  section "$2" ;;
  revision) revision "$2" ;;
  *) sed -n '2,12p' "$0"; exit 2 ;;
esac
