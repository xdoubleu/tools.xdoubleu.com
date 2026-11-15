#!/usr/bin/env bash
# Exercises scripts/lint_skills.sh against a synthetic repo: valid frontmatter
# passes, broken frontmatter fails naming the file.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail_count=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n%s\n' "$1" "$2"; fail_count=$((fail_count + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

repo="$WORK/repo"
mkdir -p "$repo/scripts" "$repo/.claude/skills/good"
cp "$ROOT_DIR/scripts/lint_skills.sh" "$repo/scripts/"
printf -- '---\nname: good\ndescription: does a thing\n---\nbody\n' > "$repo/.claude/skills/good/SKILL.md"

lint() { (cd "$repo" && ./scripts/lint_skills.sh 2>&1); }

# bad NAME CONTENT: a skill with CONTENT must fail the lint and be named.
bad() {
  local out
  mkdir -p "$repo/.claude/skills/bad"
  printf '%b' "$2" > "$repo/.claude/skills/bad/SKILL.md"
  if out=$(lint); then
    fail "$1" "expected a failure, got: $out"
  elif printf '%s' "$out" | grep -qF -- "skills/bad/SKILL.md"; then
    pass "$1"
  else
    fail "$1" "$out"
  fi
  rm -rf "$repo/.claude/skills/bad"
}

if out=$(lint); then pass "valid skill passes"; else fail "valid skill passes" "$out"; fi
bad "missing fence" 'name: bad\ndescription: x\n'
bad "invalid yaml" '---\nname: [bad\ndescription: x\n---\n'
bad "no description" '---\nname: bad\n---\n'
bad "empty name" '---\nname:\ndescription: x\n---\n'

echo "---"
if [ "$fail_count" -eq 0 ]; then
  echo "All lint_skills tests passed."
  exit 0
else
  echo "$fail_count lint_skills test(s) failed."
  exit 1
fi
