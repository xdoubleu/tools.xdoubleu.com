#!/usr/bin/env bash
# Diff-scoped mutation testing: resolves the TS/TSX lines changed vs
# origin/main via ../tools/diff_files_ts.py ("path" or "path:start-end"),
# then runs StrykerJS --mutate on just those (StrykerJS has no built-in
# git-diff scoping).
# MUTATION_DIFF_FILES (newline-separated) and STRYKER_BIN override the inputs
# for testing.
set -euo pipefail
cd "$(dirname "$0")/.."

list=${MUTATION_DIFF_FILES-$(python3 ../tools/diff_files_ts.py)}

patterns=()
while IFS= read -r entry; do
  [ -n "$entry" ] || continue
  path=$entry
  range=
  if [[ $entry =~ ^(.+)(:[0-9]+-[0-9]+)$ ]]; then
    path=${BASH_REMATCH[1]}
    range=${BASH_REMATCH[2]}
  fi
  if [ ! -f "$path" ]; then
    echo "Changed path matches no file: $path" >&2
    exit 1
  fi
  # --mutate takes globs, so escape glob metacharacters (e.g. app/[id]/...).
  escaped=$(printf '%s' "$path" | sed -e 's/[][*?{}()!]/[&]/g')
  patterns+=("$escaped$range")
done <<<"$list"

if [ ${#patterns[@]} -eq 0 ]; then
  echo "No changed TS/TSX files vs origin/main -- nothing to mutation-test."
  exit 0
fi

files=$(printf '%s\n' "${patterns[@]}" | paste -sd, -)
echo "Diff-scoped mutation testing: $files"
${STRYKER_BIN:-npx stryker} run --mutate "$files"
