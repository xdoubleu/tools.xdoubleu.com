#!/usr/bin/env bash
# Stop hook: once per new commit, blocks the stop and asks the current
# session to reflect on whether CLAUDE.md/tooling has a gap this change
# exposed, and (if so) raise it as its own issue + PR. No headless
# subprocess - this runs in the same session with the same permissions.
set -u

input=$(cat)
cwd=$(printf '%s' "$input" | jq -r '.cwd // empty')
stop_hook_active=$(printf '%s' "$input" | jq -r '.stop_hook_active // false')

# Only ever block once per stop-cycle, to avoid looping.
if [ "$stop_hook_active" = "true" ] || [ -z "$cwd" ]; then
  exit 0
fi

common_dir=$(git -C "$cwd" rev-parse --git-common-dir 2>/dev/null) || exit 0
head_sha=$(git -C "$cwd" rev-parse HEAD 2>/dev/null) || exit 0

state_dir="$common_dir/claude-reflect"
mkdir -p "$state_dir"
state_file="$state_dir/last-sha"

last_sha=""
[ -f "$state_file" ] && last_sha=$(cat "$state_file")

if [ "$last_sha" = "$head_sha" ]; then
  exit 0
fi

# Record now so this exact commit only ever triggers one reflection, even if
# the resulting reflection PR is built in a separate worktree that never
# advances HEAD here.
printf '%s' "$head_sha" > "$state_file"

range="$head_sha"
if [ -n "$last_sha" ] && git -C "$cwd" cat-file -e "$last_sha" 2>/dev/null; then
  range="$last_sha..$head_sha"
fi

reason=$(cat <<EOF
A commit just landed in this repo (range: $range). Before finishing, reflect
on whether the CLAUDE.md files in this repo, or its tooling (Makefile
targets, lint config, CI workflows, scripts, hooks) have a concrete,
non-obvious gap that this change exposed:

1. Would a documentation or tooling improvement have made this specific
   change faster or less error-prone to implement, or a related question
   easier to answer?
2. Did the pull request for this change need multiple pushes to go green -
   failed CI runs, or fixup-looking commits (fix lint, fix test, etc.)
   pushed after the PR was first opened? Check with "gh pr checks" /
   "gh run list" and the commit log. If so, what local check (a CLAUDE.md
   checklist step, a make target, a pre-push hook) would have caught it
   before pushing?

If nothing is worth flagging, say so briefly and stop - do not force a
change. If something is worthwhile: following the workflow already defined
in CLAUDE.md, use EnterWorktree to start a fresh branch off main, create a
tracking issue, edit ONLY CLAUDE.md/tooling files (never application code),
commit, push, and open a separate, non-draft PR referencing the issue - kept
independent of the PR for the code change so it can be reviewed on its own.
EOF
)

jq -n --arg reason "$reason" '{"decision":"block","reason":$reason}'
exit 0
