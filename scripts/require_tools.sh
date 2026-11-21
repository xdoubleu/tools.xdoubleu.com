#!/usr/bin/env bash
# Fail once, with the fix, when a tool the flake's dev shell supplies is not on PATH.
# Makefile targets call it as `require_tools.sh <tool>...`; nothing is installed.
set -euo pipefail

missing=()
for t in "$@"; do
	command -v "$t" >/dev/null 2>&1 || missing+=("$t")
done
[ "${#missing[@]}" -eq 0 ] && exit 0

echo "missing from PATH: ${missing[*]}" >&2
echo "tool versions come from flake.lock: run inside \`nix develop\` (or prefix with \`nix develop --command\`)." >&2
exit 1
