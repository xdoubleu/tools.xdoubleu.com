#!/usr/bin/env bash
# Fails if a macOS binary links a dylib outside the OS (e.g. /nix/store),
# which would crash it at launch on a user's Mac.
set -euo pipefail

bin="${1:?usage: check_darwin_linkage.sh <binary>}"
bad=$(otool -L "$bin" | tail -n +2 | awk '{print $1}' |
	grep -v -E '^(/System/Library/|/usr/lib/)' || true)
if [ -n "$bad" ]; then
	echo "$bin links non-system libraries (build outside nix develop):" >&2
	echo "$bad" >&2
	exit 1
fi
