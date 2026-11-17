#!/usr/bin/env bash
# Claude Code PreToolUse hook (Bash), wired in .claude/settings.json. Holds
# the rules CLAUDE.md states and an agent can forget:
#   - no commits or pushes unless the user asked: `git commit` and
#     `git push` ask the user first;
#   - no AI attribution: a commit message naming Claude as co-author is
#     refused (scripts/git-hooks/commit-msg refuses it too);
#   - no broad kills: `pkill -f` and `killall` are refused. `pkill -f cat`
#     once matched every process under /Applications.
set -uo pipefail

command -v jq >/dev/null || exit 0
cmd="$(jq -r '.tool_input.command // empty')"

decide() {
  jq -n --arg d "$1" --arg r "$2" \
    '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: $d, permissionDecisionReason: $r}}'
  exit 0
}

if grep -Eiq 'co-authored-by:.*(claude|anthropic)|generated with \[?claude' <<<"$cmd"; then
  decide deny "No AI attribution in commits or PRs (CLAUDE.md). Remove the Co-Authored-By / Generated-with line."
fi
# Only where a command starts (a line, or after ; & | or $( ), so a commit
# message or a grep that mentions them passes.
if grep -Eq '(^|[;&|]|\$\()[[:space:]]*(sudo[[:space:]]+)?(pkill[[:space:]]+(-[a-zA-Z]*f|--full)|killall)([[:space:]]|$)' <<<"$cmd"; then
  decide deny "No broad kills: find the PID you started (pgrep -fl, then check it) and kill that PID only."
fi
if grep -Eq '(^|[;&|[:space:]])git([[:space:]]+-[cC][[:space:]]+[^[:space:]]+)*[[:space:]]+(commit|push)([[:space:]]|$)' <<<"$cmd"; then
  decide ask "CLAUDE.md: no commits or pushes unless the user asked for one."
fi
exit 0
