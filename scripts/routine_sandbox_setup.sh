#!/usr/bin/env bash
# Prepares routine_sandbox.sh for an agent-routine job (Linux runner with
# passwordless sudo):
#   routine_sandbox_setup.sh <workspace> <private-dir>
# - creates the unprivileged sandbox user and lets it read and write the
#   workspace, except .git and the agent's instruction files (read-only), so
#   sandboxed code can't plant git config/hooks or edit skills;
# - makes <private-dir> (the job's RUNNER_TEMP, which holds step outputs such
#   as tokens) unreadable to it;
# - disables git hooks for the job user;
# - installs routine-sandbox and the routine scripts the job runs after the
#   agent into <private-dir>/routine-bin, out of the sandbox's reach.
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
