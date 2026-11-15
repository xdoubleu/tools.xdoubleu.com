#!/usr/bin/env bash
# Usage: ensure_go_tool.sh <binary> <module-path> <package> <version>
# go install only when <binary> isn't already on PATH built from <module-path>@v<version>,
# so offline hosts with preinstalled tools work.
set -euo pipefail
bin=$1 mod=$2 pkg=$3 ver=$4
if path=$(command -v "$bin") && go version -m "$path" 2>/dev/null |
  grep -qE "^[[:space:]]+mod[[:space:]]+${mod}[[:space:]]+v${ver}([[:space:]]|$)"; then
  exit 0
fi
go install "${pkg}@v${ver}"
