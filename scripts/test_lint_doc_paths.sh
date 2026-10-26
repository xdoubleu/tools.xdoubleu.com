#!/usr/bin/env bash
# Exercises scripts/lint_doc_paths.sh against a synthetic repo: existing paths
# pass, missing ones fail with file:line, and the historical opt-outs hold.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail_count=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n%s\n' "$1" "$2"; fail_count=$((fail_count + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

repo="$WORK/repo"
mkdir -p "$repo/scripts" "$repo/api/internal/spa" "$repo/web/src" "$repo/docs" "$repo/build"
cp "$ROOT_DIR/scripts/lint_doc_paths.sh" "$repo/scripts/"
git -C "$repo" init -q
printf 'build/\n' > "$repo/.gitignore"
touch "$repo/api/internal/spa/spa.go" "$repo/web/src/WebVitals.tsx" \
  "$repo/web/src/other.tsx" "$repo/docs/adr-0001-real.md"

lint() { (cd "$repo" && ./scripts/lint_doc_paths.sh 2>&1); }

# expect_pass NAME: the lint must exit 0. expect_fail NAME PATTERN: it must
# exit non-zero and print PATTERN.
expect_pass() {
  local out
  if out=$(lint); then pass "$1"; else fail "$1" "$out"; fi
}
expect_fail() {
  local out
  if out=$(lint); then
    fail "$1" "expected a failure, got: $out"
  elif printf '%s' "$out" | grep -qF -- "$2"; then
    pass "$1"
  else
    fail "$1" "$out"
  fi
}

# --- passing cases ---------------------------------------------------------
cat > "$repo/AGENTS.md" <<'EOF'
# Agents

Real file `api/internal/spa/spa.go`, directory `api/internal/spa/`, and line ref
`api/internal/spa/spa.go:12`. Bare name `WebVitals.tsx`, braces
`web/src/{WebVitals,other}.tsx`, route `/sw.js`, extension `.proto`.
Skipped: glob `web/src/*.ts`, placeholder `api/apps/<name>/app.go`, URL
`https://example.com/docs/x.md`, command `make lint/docs`, ignored `build/out.js`.

```
api/not/real.go is inside a fence and ignored
```
EOF
printf '# API\n\nRelative to this dir: `internal/spa/spa.go`.\n' > "$repo/api/AGENTS.md"
cat > "$repo/docs/adr-0002-old.md" <<'EOF'
# ADR

- Affects (historical): `api/gone/removed.go`, `web/old.ts`
EOF
expect_pass "existing, skipped and historical references pass"

# --- failing cases ---------------------------------------------------------
# Each case rewrites one scratch doc, so the earlier files stay valid.
scratch="$repo/docs/scratch.md"

printf '# Doc\n\nSee `api/internal/spa/missing.go` here.\n' > "$scratch"
expect_fail "a missing file in docs/*.md fails with file:line" \
  "docs/scratch.md:3: \`api/internal/spa/missing.go\` does not exist"

printf 'See `api/internal/gone/`.\n' > "$repo/AGENTS.md"
expect_fail "a missing directory in AGENTS.md fails" \
  "AGENTS.md:1: \`api/internal/gone\` does not exist"

printf 'See `build-web.yml`.\n' > "$scratch"
expect_fail "a missing bare filename fails" "\`build-web.yml\` does not exist"

printf 'See `internal/spa/missing.go`.\n' > "$repo/api/AGENTS.md"
expect_fail "a missing path in a subtree AGENTS.md fails" \
  "api/AGENTS.md:1: \`internal/spa/missing.go\` does not exist"

printf 'See `web/src/{WebVitals,missing}.tsx`.\n' > "$scratch"
expect_fail "one missing brace alternative fails" "\`web/src/missing.tsx\` does not exist"

# --- opt-out ---------------------------------------------------------------
printf '# Agents\n' > "$repo/AGENTS.md"
printf '# API\n' > "$repo/api/AGENTS.md"
printf '# Doc\n\nGone: `api/gone/a.go`, `api/gone/b.go` <!-- historical -->\n' > "$scratch"
expect_pass "a line ending in the historical marker is exempt"

printf 'Also `api/gone/c.go`.\n' >> "$scratch"
expect_fail "the marker exempts only its own line" \
  "docs/scratch.md:4: \`api/gone/c.go\` does not exist"

echo "---"
if [ "$fail_count" -eq 0 ]; then
  echo "All lint_doc_paths tests passed."
  exit 0
else
  echo "$fail_count lint_doc_paths test(s) failed."
  exit 1
fi
