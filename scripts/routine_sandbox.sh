#!/usr/bin/env bash
# Runs a command as the routine sandbox user (see routine_sandbox_setup.sh)
# with a clean environment, so dependency code it executes (npm install
# scripts, go test) can't reach this job's tokens: a different uid can't read
# the agent's /proc/<pid>/environ.
#   routine-sandbox <command> [args...]    # runs in the current directory
set -euo pipefail

user=${ROUTINE_SANDBOX_USER:-routine-sandbox}
if [ "$#" -eq 0 ]; then
  echo "usage: routine-sandbox <command> [args...]" >&2
  exit 2
fi
home=$(getent passwd "$user" | cut -d: -f6)
if [ -z "$home" ]; then
  echo "routine-sandbox: user $user missing; run routine_sandbox_setup.sh" >&2
  exit 1
fi

# Tools the sandbox installs (go install, pip --user) land in its own home.
# sudo starts from /, since the caller's directory's parents may be closed to
# the sandbox user; bash then enters the (shared) working directory.
dir=$PWD
cd /
exec sudo -n -u "$user" -- env -i \
  PATH="$home/go/bin:$home/.local/bin:$PATH" HOME="$home" USER="$user" LANG=C.UTF-8 TERM=dumb CI=true \
  bash -c 'cd "$1" && shift && exec "$@"' routine-sandbox "$dir" "$@"
