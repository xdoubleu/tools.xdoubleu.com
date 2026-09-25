#!/usr/bin/env bash
# Diff-scoped mutation testing: resolves the TS/TSX files changed vs
# origin/main via ../tools/diff_files_ts.py, then runs StrykerJS --mutate on
# just those (StrykerJS has no built-in git-diff scoping).
set -euo pipefail
cd "$(dirname "$0")/.."

files=$(python3 ../tools/diff_files_ts.py | paste -sd, -)

if [ -z "$files" ]; then
  echo "No changed TS/TSX files vs origin/main -- nothing to mutation-test."
  exit 0
fi

echo "Diff-scoped mutation testing: $files"
npx stryker run --mutate "$files"
