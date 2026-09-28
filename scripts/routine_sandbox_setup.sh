#!/usr/bin/env bash
# Prepares routine_sandbox.sh on a Linux runner with passwordless sudo:
#   routine_sandbox_setup.sh <workspace> <private-dir (RUNNER_TEMP)>
# Creates the sandbox user with workspace access minus .git and instruction
# files, hides <private-dir> from it, disables git hooks, and installs
# routine-sandbox plus the post-agent scripts into <private-dir>/routine-bin.
set -euo pipefail

workspace=$(cd "${1:?workspace}" && pwd)
private=$(cd "${2:?private dir}" && pwd)
user=${ROUTINE_SANDBOX_USER:-routine-sandbox}
me=$(id -un)
scripts=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

id "$user" >/dev/null 2>&1 || sudo useradd --create-home --shell /bin/bash "$user"
if ! command -v setfacl >/dev/null; then
  sudo apt-get update -qq && sudo apt-get install -y -qq acl
fi

chmod 700 "$private"
bin="$private/routine-bin"
mkdir -p "$bin" "$private/no-git-hooks"
install -m 755 "$scripts/routine_sandbox.sh" "$bin/routine-sandbox"
for s in routine_metrics.sh routine_record.sh routine_watchdog.sh; do
  install -m 755 "$scripts/$s" "$bin/$s"
done

# Traverse, not list, the job user's directories down to the workspace.
dir=$workspace
while dir=$(dirname "$dir") && [ "$dir" != / ]; do
  if [ -O "$dir" ]; then setfacl -m "u:$user:x" "$dir"; fi
done

setfacl -R -m "u:$user:rwX,d:u:$user:rwX,d:u:$me:rwX" "$workspace"
# Read-only for the sandbox. The workspace root's sticky bit stops it renaming
# the root ones away; subdirectories stay unsticky so tools can rename-write
# lockfiles there.
protected=()
for p in .git .claude .agents .opencode opencode.json; do
  if [ -e "$workspace/$p" ]; then protected+=("$workspace/$p"); fi
done
while IFS= read -r -d '' f; do protected+=("$f"); done < <(
  find "$workspace" -name node_modules -prune -o -name .git -prune -o \
    \( -name AGENTS.md -o -name CLAUDE.md \) -print0
)
for p in "${protected[@]}"; do
  setfacl -R -m "u:$user:rX" "$p"
  if [ -d "$p" ]; then setfacl -R -m "d:u:$user:rX" "$p"; fi
done
chmod +t "$workspace"

git config --global core.hooksPath "$private/no-git-hooks"
sudo -u "$user" -H git config --global --add safe.directory '*'
