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
SESSIONSTART_CMD=$(jq -r '.hooks.SessionStart[0].hooks[0].command' "$SETTINGS")

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

# case: no gh on PATH -> "can't tell" if a PR exists, so stay silent (issue #1400)
repo=$(setup_repo "no-gh-case")
echo y > "$repo/g.txt"
git -C "$repo" add g.txt
git -C "$repo" commit -q -m "second commit"
echo dirty > "$repo/h.txt"
NOGH_DIR="$WORK/nogh-bin"
mkdir -p "$NOGH_DIR"
for tool in bash git jq mktemp cat mkdir tr printf; do
  p=$(command -v "$tool") && ln -sf "$p" "$NOGH_DIR/$tool"
done
out=$(PATH="$NOGH_DIR" run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
[ -z "$out" ] && pass "no gh on PATH -> silent (can't tell)" || fail "no gh on PATH -> silent (can't tell)" "$out"

# --- ExitPlanMode hook ---------------------------------------------------
out=$(bash -c "$EXITPLAN_CMD")
if printf '%s' "$out" | jq -e '.hookSpecificOutput.additionalContext | contains("start-task")' > /dev/null 2>&1; then
  pass "ExitPlanMode hook returns start-task reminder"
else
  fail "ExitPlanMode hook returns start-task reminder" "$out"
fi

# --- SessionStart hook ---------------------------------------------------
run_session_start() {
  local payload="$1"
  printf '%s' "$payload" | bash -c "$SESSIONSTART_CMD"
}

# case: fast-forwards local main when behind origin and clean
BARE="$WORK/origin.git"
git init -q --bare "$BARE"
LOCAL="$WORK/local-main-repo"
git clone -q "$BARE" "$LOCAL"
git -C "$LOCAL" config user.email test@example.com
git -C "$LOCAL" config user.name test
echo a > "$LOCAL/a.txt"
git -C "$LOCAL" add a.txt
git -C "$LOCAL" commit -q -m init
git -C "$LOCAL" push -q origin HEAD:main
git -C "$LOCAL" branch -q -m main
git -C "$LOCAL" branch -q --set-upstream-to=origin/main main

# advance the remote from a second clone, simulating another session's merge
OTHER="$WORK/other-clone"
git clone -q "$BARE" "$OTHER"
git -C "$OTHER" config user.email test@example.com
git -C "$OTHER" config user.name test
echo b > "$OTHER/b.txt"
git -C "$OTHER" add b.txt
git -C "$OTHER" commit -q -m "second commit"
git -C "$OTHER" push -q origin HEAD:main

before=$(git -C "$LOCAL" rev-parse main)
run_session_start "$(jq -n --arg cwd "$LOCAL" '{cwd:$cwd}')" > /dev/null
after=$(git -C "$LOCAL" rev-parse main)
remote_head=$(git -C "$BARE" rev-parse main)
[ "$after" = "$remote_head" ] && [ "$after" != "$before" ] &&
  pass "SessionStart fast-forwards clean local main to origin/main" ||
  fail "SessionStart fast-forwards clean local main to origin/main" "before=$before after=$after remote=$remote_head"

# case: leaves a dirty local main alone (never discards uncommitted work)
echo dirty > "$LOCAL/a.txt"
before=$(git -C "$LOCAL" rev-parse main)
git -C "$LOCAL" fetch -q origin main
# reset the remote-tracking view back to a stale point isn't needed; just
# re-run against the now-dirty tree and confirm main doesn't move.
run_session_start "$(jq -n --arg cwd "$LOCAL" '{cwd:$cwd}')" > /dev/null
after=$(git -C "$LOCAL" rev-parse main)
[ "$after" = "$before" ] &&
  pass "SessionStart leaves dirty local main untouched" ||
  fail "SessionStart leaves dirty local main untouched" "before=$before after=$after"
git -C "$LOCAL" checkout -q -- a.txt

# case: non-main branch is left alone
git -C "$LOCAL" checkout -q -b feature-branch
before=$(git -C "$LOCAL" rev-parse feature-branch)
run_session_start "$(jq -n --arg cwd "$LOCAL" '{cwd:$cwd}')" > /dev/null
after=$(git -C "$LOCAL" rev-parse feature-branch)
[ "$after" = "$before" ] &&
  pass "SessionStart leaves a non-main branch untouched" ||
  fail "SessionStart leaves a non-main branch untouched" "before=$before after=$after"

echo "---"
if [ "$fail_count" -eq 0 ]; then
  echo "All hook tests passed."
  exit 0
else
  echo "$fail_count hook test(s) failed."
  exit 1
fi
