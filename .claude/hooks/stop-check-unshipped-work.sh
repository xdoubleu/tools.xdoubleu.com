#!/bin/bash
# Blocks Stop when the worktree has unshipped work (commits ahead of
# origin/main, or uncommitted changes) and no PR exists for the branch.
# See docs/adr-0014-start-finish-task-enforcement.md.
set -uo pipefail

input=$(cat)
stop_active=$(printf '%s' "$input" | jq -r '.stop_hook_active // false')
if [ "$stop_active" = "true" ]; then
  exit 0
fi

cwd=$(printf '%s' "$input" | jq -r '.cwd // empty')
if [ -z "$cwd" ]; then
  exit 0
fi
case "$cwd" in
  */.claude/worktrees/*) ;;
  *) exit 0 ;;
esac

branch=$(git -C "$cwd" rev-parse --abbrev-ref HEAD 2>/dev/null) || exit 0
if [ "$branch" = "main" ] || [ "$branch" = "HEAD" ]; then
  exit 0
fi

ahead=$(git -C "$cwd" rev-list --count origin/main..HEAD 2>/dev/null || echo 0)
dirty=$(git -C "$cwd" status --porcelain 2>/dev/null | head -1)
if [ "$ahead" = "0" ] && [ -z "$dirty" ]; then
  exit 0
fi
if [ -n "$dirty" ]; then
  state=dirty
else
  state=clean
fi

# --- Does a PR already exist for this branch? ---------------------------
# Prefer `gh`; otherwise call the REST API with the credential
# `git credential fill` resolves. If neither can tell, don't block.
prs=""
if command -v gh >/dev/null 2>&1; then
  prs=$(cd "$cwd" && gh pr list --head "$branch" --state all --json number --jq length 2>/dev/null)
elif command -v curl >/dev/null 2>&1; then
  origin_url=$(git -C "$cwd" remote get-url origin 2>/dev/null)
  owner_repo=$(printf '%s' "$origin_url" | sed -E 's#^(https://github\.com/|git@github\.com:)##; s#\.git$##')
  owner="${owner_repo%%/*}"
  token=$(printf 'protocol=https\nhost=github.com\n\n' | git -C "$cwd" credential fill 2>/dev/null | sed -n 's/^password=//p')
  if [ -n "$owner_repo" ] && [ -n "$owner" ] && [ -n "$token" ]; then
    prs=$(curl -sf \
      -H "Authorization: Bearer $token" \
      -H "Accept: application/vnd.github+json" \
      "https://api.github.com/repos/$owner_repo/pulls?head=$owner:$branch&state=all" \
      2>/dev/null | jq -r 'length' 2>/dev/null)
  fi
fi
if [ -z "$prs" ] || [ "$prs" != "0" ]; then
  exit 0
fi

common=$(git -C "$cwd" rev-parse --path-format=absolute --git-common-dir 2>/dev/null) || exit 0
head_sha=$(git -C "$cwd" rev-parse HEAD 2>/dev/null) || exit 0
sd="$common/claude-finish-task"
mkdir -p "$sd"
sf="$sd/$(printf '%s' "$branch" | tr '/' '-')"
last=""
[ -f "$sf" ] && last=$(cat "$sf")
key="$head_sha-$state"
if [ "$last" = "$key" ]; then
  exit 0
fi
printf '%s' "$key" > "$sf"
jq -n --arg b "$branch" '{"decision":"block","reason":("Branch \($b) has finished work that was never shipped: commits ahead of origin/main (or uncommitted changes), and no pull request for it. Run the finish-task skill now - lint, coverage, the web build, then ship-pr to open the PR and watch CI to green, then session-retro. Opening the PR is standing pre-authorized workflow for this repo: do NOT wait to be asked, and do NOT stop at having pushed the branch. If the work genuinely is not finished, say what is left and stop - this hook will not fire again for the same commit.")}'
