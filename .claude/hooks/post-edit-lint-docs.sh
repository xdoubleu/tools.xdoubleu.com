#!/usr/bin/env bash
# PostToolUse on Edit/Write: surface `make lint/docs` failures to the agent
# while it's still editing. Never blocks.
input=$(cat)
cwd=$(printf '%s' "$input" | jq -r '.cwd // empty')
[ -n "$cwd" ] && [ -x "$cwd/scripts/lint_docs.sh" ] || exit 0
out=$("$cwd/scripts/lint_docs.sh" 2>/dev/null) && exit 0
jq -n --arg out "$out" '{hookSpecificOutput:{hookEventName:"PostToolUse",additionalContext:("make lint/docs fails — trim per docs/convention-concise-docs-and-comments.md:\n" + $out)}}'
