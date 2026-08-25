#!/usr/bin/env bash
# Exercises the branching hooks in .claude/settings.json against synthetic
# payloads/repos, so a regression fails loudly instead of staying silent.
# Invoked via `make hooks/test` — see root Makefile.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SETTINGS="$ROOT_DIR/.claude/settings.json"

fail_count=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n%s\n' "$1" "$2"; fail_count=$((fail_count + 1)); }

# --- settings.json sanity ---------------------------------------------
if ! jq -e . "$SETTINGS" > /dev/null 2>&1; then
  fail "settings.json parses as JSON" "jq -e . failed"
  exit 1
fi
pass "settings.json parses as JSON"

while IFS= read -r cmd; do
  if ! bash -n <(printf '%s' "$cmd") 2>/dev/null; then
    fail "hook command passes bash -n" "$cmd"
  fi
done < <(jq -r '[.hooks[][].hooks[]?.command] | .[]' "$SETTINGS")
pass "all hook commands pass bash -n"

STOP_CMD=$(jq -r '.hooks.Stop[0].hooks[0].command' "$SETTINGS")
EXITPLAN_CMD=$(jq -r '.hooks.PostToolUse[] | select(.matcher=="ExitPlanMode") | .hooks[0].command' "$SETTINGS")

# --- synthetic worktree-shaped repo ------------------------------------
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

setup_repo() {
  local branch="$1"
  local repo="$WORK/.claude/worktrees/$branch"
  rm -rf "$repo"
  mkdir -p "$repo"
  git init -q -b "$branch" "$repo"
  git -C "$repo" config user.email test@example.com
  git -C "$repo" config user.name test
  echo x > "$repo/f.txt"
  git -C "$repo" add f.txt
  git -C "$repo" commit -q -m init
  # fabricate an origin/main ref pointing at the same commit
  git -C "$repo" update-ref refs/remotes/origin/main HEAD
  printf '%s' "$repo"
}

run_stop() {
  local payload="$1"
  printf '%s' "$payload" | bash -c "$STOP_CMD"
}

# stub gh so the "existing PR" case doesn't hit the network
STUB_DIR="$WORK/stub-bin"
mkdir -p "$STUB_DIR"

gh_stub() {
  local count="$1"
  cat > "$STUB_DIR/gh" <<EOF
#!/usr/bin/env bash
echo "$count"
EOF
  chmod +x "$STUB_DIR/gh"
}

# case: stop_hook_active suppresses everything
repo=$(setup_repo "loop-guard")
out=$(run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:true}')")
[ -z "$out" ] && pass "stop_hook_active=true is silent" || fail "stop_hook_active=true is silent" "$out"

# case: cwd not under .claude/worktrees/
out=$(run_stop "$(jq -n '{cwd:"/tmp/not-a-worktree", stop_hook_active:false}')")
[ -z "$out" ] && pass "non-worktree cwd is silent" || fail "non-worktree cwd is silent" "$out"

# case: empty cwd
out=$(run_stop "$(jq -n '{cwd:"", stop_hook_active:false}')")
[ -z "$out" ] && pass "empty cwd is silent" || fail "empty cwd is silent" "$out"

# case: branch is main
repo=$(setup_repo "main")
out=$(run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
[ -z "$out" ] && pass "branch=main is silent" || fail "branch=main is silent" "$out"

# case: clean tree, 0 commits ahead
repo=$(setup_repo "clean-case")
out=$(run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
[ -z "$out" ] && pass "clean tree / 0 ahead is silent" || fail "clean tree / 0 ahead is silent" "$out"

# case: commits ahead + existing PR
repo=$(setup_repo "has-pr")
echo y > "$repo/g.txt"
git -C "$repo" add g.txt
git -C "$repo" commit -q -m "second commit"
gh_stub 1
out=$(PATH="$STUB_DIR:$PATH" run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
[ -z "$out" ] && pass "commits ahead with existing PR is silent" || fail "commits ahead with existing PR is silent" "$out"

# case: fires — commits ahead, no PR, dirty tree
repo=$(setup_repo "fires-case")
echo y > "$repo/g.txt"
git -C "$repo" add g.txt
git -C "$repo" commit -q -m "second commit"
echo dirty > "$repo/h.txt"
gh_stub 0
out=$(PATH="$STUB_DIR:$PATH" run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
if printf '%s' "$out" | jq -e '.decision == "block"' > /dev/null 2>&1; then
  pass "commits ahead, no PR, dirty -> fires block decision"
else
  fail "commits ahead, no PR, dirty -> fires block decision" "$out"
fi

# case: per-SHA suppression on second invocation
out2=$(PATH="$STUB_DIR:$PATH" run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
[ -z "$out2" ] && pass "second invocation for same SHA is suppressed" || fail "second invocation for same SHA is suppressed" "$out2"

# --- ExitPlanMode hook ---------------------------------------------------
out=$(bash -c "$EXITPLAN_CMD")
if printf '%s' "$out" | jq -e '.hookSpecificOutput.additionalContext | contains("start-task")' > /dev/null 2>&1; then
  pass "ExitPlanMode hook returns start-task reminder"
else
  fail "ExitPlanMode hook returns start-task reminder" "$out"
fi

echo "---"
if [ "$fail_count" -eq 0 ]; then
  echo "All hook tests passed."
  exit 0
else
  echo "$fail_count hook test(s) failed."
  exit 1
fi
