#!/usr/bin/env bash
# Exercises check_scalar_subqueries.sh against a synthetic api/ tree.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check_scalar_subqueries.sh"

fail_count=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n%s\n' "$1" "$2"; fail_count=$((fail_count + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

api="$WORK/api"
mkdir -p "$api/apps/demo/migrations" "$api/apps/demo/internal" "$api/scripts"
cp "$SCRIPT" "$api/scripts/"
cat > "$api/apps/demo/migrations/00001.sql" <<'SQL'
-- +goose Up
CREATE TABLE IF NOT EXISTS demo.links (
    url TEXT PRIMARY KEY,
    task_id UUID NOT NULL,
    note TEXT
);
CREATE TABLE IF NOT EXISTS demo.users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE
);
-- +goose Down
DROP TABLE IF EXISTS demo.links;
SQL
cat > "$api/apps/demo/migrations/00002.sql" <<'SQL'
-- +goose Up
ALTER TABLE demo.links DROP CONSTRAINT IF EXISTS links_pkey;
ALTER TABLE demo.links ADD PRIMARY KEY (url, task_id);
-- +goose Down
ALTER TABLE demo.links DROP CONSTRAINT IF EXISTS links_pkey;
ALTER TABLE demo.links ADD PRIMARY KEY (url);
SQL

put() { printf '%s\n' "$2" > "$api/apps/demo/internal/case.go"; }
check() { (cd "$api" && ./scripts/check_scalar_subqueries.sh 2>&1); }
expect_pass() {
  local out
  if out=$(check); then pass "$1"; else fail "$1" "$out"; fi
}
expect_fail() {
  local out
  if out=$(check); then
    fail "$1" "expected a failure, got: $out"
  elif printf '%s' "$out" | grep -qF -- "$2"; then
    pass "$1"
  else
    fail "$1" "$out"
  fi
}

put x 'const q = `SELECT (SELECT note FROM demo.links WHERE url = $1)`'
expect_fail 'key narrowed by a later migration' 'case.go:1: scalar subquery on demo.links'

put x 'const q = `SELECT (SELECT note FROM demo.links WHERE url = $1 AND task_id = $2)`'
expect_pass 'full primary key'

put x 'const q = `SELECT (SELECT note FROM demo.links WHERE url = $1 LIMIT 1)`'
expect_pass 'LIMIT 1'

put x 'const q = `SELECT (SELECT count(*) FROM demo.links WHERE url = $1)`'
expect_pass 'aggregate'

put x 'const q = `SELECT (SELECT id FROM demo.users WHERE email = $1)`'
expect_pass 'unique column'

put x 'const q = `SELECT (SELECT id FROM demo.users WHERE id <> $1)`'
expect_fail 'inequality is no key' 'scalar subquery on demo.users'

put x 'const q = `SELECT 1 WHERE EXISTS (SELECT 1 FROM demo.links) AND x IN (SELECT url FROM demo.links)`'
expect_pass 'EXISTS and IN'

put x 'const q = `SELECT (SELECT note FROM demo.links -- single-row
)`'
expect_pass 'single-row marker'

printf '%s\n' 'const q = `SELECT (SELECT note FROM demo.links)`' > "$api/apps/demo/internal/case_test.go"
rm "$api/apps/demo/internal/case.go"
expect_pass 'test files are exempt'

exit $((fail_count > 0))
