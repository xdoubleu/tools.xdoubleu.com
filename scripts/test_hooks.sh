#!/usr/bin/env bash
# Exercises the hooks in .claude/settings.json against synthetic repos.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SETTINGS="$ROOT_DIR/.claude/settings.json"
# The Stop hook path is $CLAUDE_PROJECT_DIR-relative, as in the real harness.
export CLAUDE_PROJECT_DIR="$ROOT_DIR"

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
EDITGUARD_CMD=$(jq -r '.hooks.PreToolUse[] | select(.matcher=="Edit|Write|NotebookEdit") | .hooks[0].command' "$SETTINGS")

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
  # a real-shaped origin URL, so the no-gh REST fallback can derive owner/repo
  git -C "$repo" remote add origin "https://github.com/test-owner/test-repo.git"
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

# Kept separate from STUB_DIR (which gains a `gh` stub) so it stays gh-less.
NOGH_STUB_DIR="$WORK/nogh-stub-bin"
mkdir -p "$NOGH_STUB_DIR"

# stub curl so the no-gh REST-API fallback doesn't hit the network
curl_stub() {
  local body="$1"
  cat > "$NOGH_STUB_DIR/curl" <<EOF
#!/usr/bin/env bash
cat <<'BODY'
$body
BODY
EOF
  chmod +x "$NOGH_STUB_DIR/curl"
}

# Stands in for a real session's git credential helper.
credential_stub() {
  local repo="$1"
  cat > "$NOGH_STUB_DIR/git-credential-stub" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = "get" ]; then
  echo "username=stub-user"
  echo "password=stub-token"
fi
EOF
  chmod +x "$NOGH_STUB_DIR/git-credential-stub"
  git -C "$repo" config credential.helper "$NOGH_STUB_DIR/git-credential-stub"
}

# Real git/jq but no `gh`, like a cloud/routine session.
NOGH_WITH_CURL_DIR="$WORK/nogh-with-curl-bin"
mkdir -p "$NOGH_WITH_CURL_DIR"
for tool in bash git jq mktemp cat mkdir tr printf sed head; do
  p=$(command -v "$tool") && ln -sf "$p" "$NOGH_WITH_CURL_DIR/$tool"
done

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

# case: no gh AND no curl on PATH -> truly "can't tell", so stay silent
repo=$(setup_repo "no-gh-no-curl-case")
echo y > "$repo/g.txt"
git -C "$repo" add g.txt
git -C "$repo" commit -q -m "second commit"
echo dirty > "$repo/h.txt"
NOGH_DIR="$WORK/nogh-bin"
mkdir -p "$NOGH_DIR"
for tool in bash git jq mktemp cat mkdir tr printf head; do
  p=$(command -v "$tool") && ln -sf "$p" "$NOGH_DIR/$tool"
done
out=$(PATH="$NOGH_DIR" run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
[ -z "$out" ] && pass "no gh, no curl -> silent (can't tell)" || fail "no gh, no curl -> silent (can't tell)" "$out"

# case: no gh, REST fallback finds an existing PR -> silent
repo=$(setup_repo "no-gh-rest-has-pr-case")
echo y > "$repo/g.txt"
git -C "$repo" add g.txt
git -C "$repo" commit -q -m "second commit"
echo dirty > "$repo/h.txt"
credential_stub "$repo"
curl_stub '[{"number":1}]'
out=$(PATH="$NOGH_STUB_DIR:$NOGH_WITH_CURL_DIR" run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
[ -z "$out" ] && pass "no gh, REST fallback finds existing PR -> silent" || fail "no gh, REST fallback finds existing PR -> silent" "$out"

# case: no gh, REST fallback finds no PR -> fires
repo=$(setup_repo "no-gh-rest-fires-case")
echo y > "$repo/g.txt"
git -C "$repo" add g.txt
git -C "$repo" commit -q -m "second commit"
echo dirty > "$repo/h.txt"
credential_stub "$repo"
curl_stub '[]'
out=$(PATH="$NOGH_STUB_DIR:$NOGH_WITH_CURL_DIR" run_stop "$(jq -n --arg cwd "$repo" '{cwd:$cwd, stop_hook_active:false}')")
if printf '%s' "$out" | jq -e '.decision == "block"' > /dev/null 2>&1; then
  pass "no gh, REST fallback finds no PR -> fires block decision"
else
  fail "no gh, REST fallback finds no PR -> fires block decision" "$out"
fi

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

# advance the remote from a second clone
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

# --- Edit/Write/NotebookEdit worktree-scope guard -----------------------
run_editguard() {
  local payload="$1"
  printf '%s' "$payload" | bash -c "$EDITGUARD_CMD"
}

wt=$(setup_repo "guard-case")
repo_root="${wt%/.claude/worktrees/guard-case}"

# case: file_path under the active worktree is allowed
out=$(run_editguard "$(jq -n --arg cwd "$wt" --arg fp "$wt/docs/x.md" '{cwd:$cwd, tool_input:{file_path:$fp}}')")
[ -z "$out" ] && pass "edit inside active worktree is silent" || fail "edit inside active worktree is silent" "$out"

# case: file_path in the main checkout is denied
out=$(run_editguard "$(jq -n --arg cwd "$wt" --arg fp "$repo_root/docs/x.md" '{cwd:$cwd, tool_input:{file_path:$fp}}')")
if printf '%s' "$out" | jq -e '.hookSpecificOutput.permissionDecision == "deny"' > /dev/null 2>&1; then
  pass "edit in the main checkout -> denied"
else
  fail "edit in the main checkout -> denied" "$out"
fi

# case: file_path in a sibling worktree of the same repo is denied
out=$(run_editguard "$(jq -n --arg cwd "$wt" --arg fp "$repo_root/.claude/worktrees/other-case/docs/x.md" '{cwd:$cwd, tool_input:{file_path:$fp}}')")
if printf '%s' "$out" | jq -e '.hookSpecificOutput.permissionDecision == "deny"' > /dev/null 2>&1; then
  pass "edit in a sibling worktree -> denied"
else
  fail "edit in a sibling worktree -> denied" "$out"
fi

# case: notebook_path field (NotebookEdit) is checked the same way
out=$(run_editguard "$(jq -n --arg cwd "$wt" --arg fp "$repo_root/nb.ipynb" '{cwd:$cwd, tool_input:{notebook_path:$fp}}')")
if printf '%s' "$out" | jq -e '.hookSpecificOutput.permissionDecision == "deny"' > /dev/null 2>&1; then
  pass "NotebookEdit outside the worktree -> denied"
else
  fail "NotebookEdit outside the worktree -> denied" "$out"
fi

# case: file outside the repo entirely is left alone (e.g. a /tmp scratch file)
out=$(run_editguard "$(jq -n --arg cwd "$wt" '{cwd:$cwd, tool_input:{file_path:"/tmp/scratch.txt"}}')")
[ -z "$out" ] && pass "edit outside the repo entirely is silent" || fail "edit outside the repo entirely is silent" "$out"

# case: cwd not under .claude/worktrees/ and not even a repo never fires
out=$(run_editguard "$(jq -n '{cwd:"/tmp/not-a-worktree", tool_input:{file_path:"/tmp/not-a-worktree/x.md"}}')")
[ -z "$out" ] && pass "edit guard silent outside any repo" || fail "edit guard silent outside any repo" "$out"

# case: cwd is the main checkout (never entered a worktree)
main_repo="$WORK/main-checkout"
mkdir -p "$main_repo"
git init -q -b main "$main_repo"
git -C "$main_repo" config user.email test@example.com
git -C "$main_repo" config user.name test
echo x > "$main_repo/f.txt"
git -C "$main_repo" add f.txt
git -C "$main_repo" commit -q -m init

out=$(run_editguard "$(jq -n --arg cwd "$main_repo" --arg fp "$main_repo/docs/x.md" '{cwd:$cwd, tool_input:{file_path:$fp}}')")
if printf '%s' "$out" | jq -e '.hookSpecificOutput.permissionDecision == "deny"' > /dev/null 2>&1; then
  pass "edit in the main checkout of a repo, never having entered a worktree -> denied"
else
  fail "edit in the main checkout of a repo, never having entered a worktree -> denied" "$out"
fi

echo "---"
if [ "$fail_count" -eq 0 ]; then
  echo "All hook tests passed."
  exit 0
else
  echo "$fail_count hook test(s) failed."
  exit 1
fi
