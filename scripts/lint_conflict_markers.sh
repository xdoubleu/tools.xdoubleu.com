#!/usr/bin/env bash
# Fail if any git-tracked file has a leftover merge-conflict marker (issue
# #1583). A stray `>>>>>>> <sha> (...)` line sat undetected in
# docs/adr-0022-prometheus-grafana-metrics.md on main for a while — nothing
# in this repo's tooling scans for these, so a bad manual conflict
# resolution merges silently.
#
# Only checks for `<<<<<<< ` and `>>>>>>> ` (with a trailing space, git's own
# marker format). Deliberately not `=======` — that line alone is valid
# Markdown h1-underline syntax and would false-positive on every doc using
# it. No path exclusions: a conflict marker is never legitimate in a
# generated file either, and `git grep` already only searches tracked text
# files.
#
# Run via `make lint/conflict-markers`; also run by main.yml's
# conflict-markers job (unconditionally — a marker can land in any file).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if matches="$(git grep -n -E '^(<{7} |>{7} )' -- .)"; then
  echo "error: leftover git conflict marker(s) found:" >&2
  echo "$matches" >&2
  exit 1
fi
echo "ok: no leftover conflict markers"
