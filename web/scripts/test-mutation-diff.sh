#!/usr/bin/env bash
# Diff-scoped mutation testing (issue #1632): resolves the changed,
# coverage-relevant TS/TSX files vs origin/main via
# ../tools/diff_files_ts.py (reusing diff_coverage_ts.py's
# get_changed_lines -- the same git-diff-against-origin/main mechanism
# adr-0013's diff-scoped coverage already established), then runs
# StrykerJS's --mutate scoped to just those files. StrykerJS's CLI has no
# built-in git-diff-based scoping in the installed version (`--since`
# doesn't exist; `--incremental` only speeds up reruns of an already-known
# mutant set), so this is how "diff-scoped" is achieved here.
set -euo pipefail
cd "$(dirname "$0")/.."

files=$(python3 ../tools/diff_files_ts.py | paste -sd, -)

if [ -z "$files" ]; then
  echo "No changed TS/TSX files vs origin/main -- nothing to mutation-test."
  exit 0
fi

echo "Diff-scoped mutation testing: $files"
npx stryker run --mutate "$files"
