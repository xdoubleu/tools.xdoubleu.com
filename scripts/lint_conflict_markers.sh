#!/usr/bin/env bash
# Fail if any tracked file has a merge-conflict marker. Checks only
# `<<<<<<< ` and `>>>>>>> `: `=======` is valid Markdown.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if matches="$(git grep -n -E '^(<{7} |>{7} )' -- .)"; then
  echo "error: leftover git conflict marker(s) found:" >&2
  echo "$matches" >&2
  exit 1
fi
echo "ok: no leftover conflict markers"
