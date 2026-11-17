#!/usr/bin/env bash
# Claude Code PostToolUse hook (Edit|Write|MultiEdit), wired in
# .claude/settings.json: after an agent edits a Go file, format it and vet
# its package, so a mistake is reported on the edit that made it rather
# than at the end of the task. Exit 2 hands the message back to the agent.
set -uo pipefail

command -v jq >/dev/null || exit 0
file="$(jq -r '.tool_input.file_path // empty')"
[[ "$file" == *.go && -f "$file" ]] || exit 0
case "$file" in
  */temp_testdata/*|*/testdata/*|*/.claude/worktrees/*) exit 0 ;;
esac

gofmt -w "$file" 2>/dev/null
dir="$(dirname "$file")"
if ! out="$(cd "$dir" && go vet . 2>&1)"; then
  echo "go vet failed in $dir after editing $(basename "$file"):" >&2
  echo "$out" | grep -v '^#' | head -20 >&2
  exit 2
fi
exit 0
