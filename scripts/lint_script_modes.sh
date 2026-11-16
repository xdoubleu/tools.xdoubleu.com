#!/usr/bin/env bash
# Fail when a script lost its executable bit (e.g. `sed -i` rewrote it):
# a `./scripts/*.sh` run by a Makefile, or a tracked *.sh that the base
# (origin/main, else HEAD~1) had as 100755, must be executable.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

bad=()
check() {
  local f="$1"
  [ -f "$f" ] || return 0
  if [ ! -x "$f" ] || [ "$(git ls-files -s -- "$f" | cut -c1-6)" != 100755 ]; then
    bad+=("$f")
  fi
}

while IFS= read -r f; do check "$f"; done < <(
  git grep -h -o -E '\./scripts/[A-Za-z0-9_./-]+\.sh' -- '*Makefile' 'Makefile' | sed 's|^\./||' | sort -u
)

base=""
for ref in origin/main HEAD~1; do
  if git rev-parse --verify -q "$ref" >/dev/null; then base="$ref"; break; fi
done
if [ -n "$base" ]; then
  while IFS= read -r f; do check "$f"; done < <(
    git ls-tree -r "$base" | awk '$1 == "100755" && $4 ~ /\.sh$/ {print $4}'
  )
fi

if [ "${#bad[@]}" -gt 0 ]; then
  printf 'error: not executable (restore with chmod +x and git update-index --chmod=+x): %s\n' "${bad[@]}" >&2
  exit 1
fi
echo "ok: script modes"
