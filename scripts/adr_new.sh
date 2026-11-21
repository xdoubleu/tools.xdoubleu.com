#!/usr/bin/env bash
# Creates the next ADR from docs/TEMPLATE-adr.md and adds its line to the AGENTS.md
# index. The number is the highest of the working tree and origin/main, plus one,
# so concurrent branches pick distinct numbers (fetching first narrows the race).
# Usage: scripts/adr_new.sh <slug> [index label]
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

slug="${1:-}"
label="${2:-$slug}"
if [[ ! "$slug" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]]; then
  echo "usage: $0 <kebab-case-slug> [index label]" >&2
  exit 2
fi

base="${BASE_REF:-origin/main}"
git fetch --quiet origin main 2>/dev/null || echo "adr_new: fetch failed, using local $base" >&2

max=0
for name in $( (ls docs; git ls-tree --name-only "$base" docs/ 2>/dev/null | sed 's|^docs/||') | sort -u); do
  if [[ "$name" =~ ^adr-([0-9]{4})- ]]; then
    n=$((10#${BASH_REMATCH[1]}))
    ((n > max)) && max=$n
  fi
done
num="$(printf '%04d' $((max + 1)))"
file="docs/adr-${num}-${slug}.md"

sed "s/ADR-NNNN: /ADR-${num}: /" docs/TEMPLATE-adr.md >"$file"
python3 - "$num" "$file" "$label" <<'PY'
import sys
num, file, label = sys.argv[1:]
lines = open("AGENTS.md").read().split("\n")
last = max(i for i, l in enumerate(lines) if l.startswith("  - [") and "(docs/adr-" in l)
lines.insert(last + 1, f"  - [{num}]({file}) {label}")
open("AGENTS.md", "w").write("\n".join(lines))
PY
echo "$file"
