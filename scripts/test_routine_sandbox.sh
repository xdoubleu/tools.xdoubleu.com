#!/usr/bin/env bash
# Exercises routine_sandbox_setup.sh + routine_sandbox.sh: sandboxed commands
# must not see the job's env or the agent's /proc environ, must not touch
# .git, instruction files or the private dir, and must still work in the
# workspace. Needs Linux with passwordless sudo; skips elsewhere.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ "$(uname -s)" != Linux ] || ! sudo -n true 2>/dev/null; then
  echo "SKIP: needs Linux with passwordless sudo"
  exit 0
fi

fail=0
pass() { printf 'PASS: %s\n' "$1"; }
failn() { printf 'FAIL: %s\n' "$1"; fail=1; }
check() { if eval "$2"; then pass "$1"; else failn "$1"; fi; }

export ROUTINE_SANDBOX_USER="routine-sandbox-test-$$"
WORK=$(mktemp -d)
cleanup() {
  sudo userdel -r "$ROUTINE_SANDBOX_USER" >/dev/null 2>&1
  sudo rm -rf "$WORK"
}
trap cleanup EXIT
chmod 755 "$WORK"

ws="$WORK/work/repo"
private="$WORK/temp"
mkdir -p "$ws/.git" "$ws/.claude/skills" "$ws/api" "$private"
echo "[core]" > "$ws/.git/config"
echo "skill" > "$ws/.claude/skills/SKILL.md"
echo "rules" > "$ws/AGENTS.md"
echo "rules" > "$ws/api/AGENTS.md"
echo "token=s3cret" > "$private/output"

# Global git config goes to a throwaway HOME, not the caller's.
HOME="$WORK" "$ROOT_DIR/scripts/routine_sandbox_setup.sh" "$ws" "$private" \
  || { echo "FAIL: setup"; exit 1; }
sandbox="$private/routine-bin/routine-sandbox"

check "setup installs the wrapper and post-agent scripts" \
  '[ -x "$sandbox" ] && [ -x "$private/routine-bin/routine_record.sh" ]'
check "setup disables git hooks for the job user" \
  '[ "$(HOME="$WORK" git config --global core.hooksPath)" = "$private/no-git-hooks" ]'

cd "$ws" || exit 1
check "runs as the sandbox user" \
  '[ "$(SECRET_TOKEN=x "$sandbox" id -un)" = "$ROUTINE_SANDBOX_USER" ]'
check "runs in the caller directory" \
  '[ "$(cd api && "$sandbox" pwd)" = "$ws/api" ]'
check "drops the job environment" \
  '[ -z "$(SECRET_TOKEN=s3cret "$sandbox" printenv SECRET_TOKEN)" ]'

SECRET_TOKEN=s3cret sleep 30 &
agent=$!
check "cannot read the agent's /proc environ" \
  '! "$sandbox" cat "/proc/$agent/environ" 2>/dev/null | grep -q s3cret'
kill "$agent" 2>/dev/null

check "cannot read the private dir" '! "$sandbox" cat "$private/output" 2>/dev/null'
check "can write the workspace" '"$sandbox" sh -c "echo built > api/out.txt"'
check "job user can edit sandbox-written files" 'echo edited >> api/out.txt'
check "cannot write .git" \
  '! "$sandbox" sh -c "echo evil >> .git/config" 2>/dev/null && ! grep -q evil .git/config'
check "cannot rename .git" '! "$sandbox" mv .git .git-old 2>/dev/null && [ -d .git ]'
check "cannot edit skills" \
  '! "$sandbox" sh -c "echo evil >> .claude/skills/SKILL.md" 2>/dev/null'
check "cannot replace the root AGENTS.md" \
  '! "$sandbox" rm -f AGENTS.md 2>/dev/null && [ -f AGENTS.md ]'
check "cannot edit a nested AGENTS.md" \
  '! "$sandbox" sh -c "echo evil >> api/AGENTS.md" 2>/dev/null'
check "can rename-write files in subdirectories (lockfiles)" \
  'echo "{}" > api/lock.json && "$sandbox" sh -c "echo new > api/.tmp && mv api/.tmp api/lock.json"'
check "puts the sandbox's own tool dirs on PATH" \
  '"$sandbox" printenv PATH | grep -q "^/home/$ROUTINE_SANDBOX_USER/go/bin:"'
check "rejects a missing command" '! "$sandbox" 2>/dev/null'

exit "$fail"
