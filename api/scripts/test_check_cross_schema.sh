#!/usr/bin/env bash
# Exercises check_cross_schema.sh against a synthetic api/ tree: downstream
# reads and FKs pass; upstream, sideways and unlisted writes fail with
# file:line; test files and comments are ignored.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check_cross_schema.sh"

fail_count=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n%s\n' "$1" "$2"; fail_count=$((fail_count + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

api="$WORK/api"
for app in recipes mealplans shoppinglist books feeds; do
  mkdir -p "$api/apps/$app/migrations" "$api/apps/$app/internal/repositories"
done
mkdir -p "$api/scripts"
cp "$SCRIPT" "$api/scripts/"

# put FILE CONTENT: (re)write a file under the synthetic api/.
put() { printf '%s\n' "$2" > "$api/$1"; }
clear_case() { rm -f "$api"/apps/*/internal/repositories/case* "$api"/apps/*/migrations/case*; }

check() { (cd "$api" && ./scripts/check_cross_schema.sh 2>&1); }

# expect_pass NAME. expect_fail NAME PATTERN: non-zero exit printing PATTERN.
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

repo=apps/shoppinglist/internal/repositories/case.go
plans=apps/mealplans/internal/repositories/case.go

clear_case
put "$repo" 'const q = `SELECT 1 FROM mealplans.plans p JOIN recipes.recipes r ON true`'
put apps/shoppinglist/migrations/case.sql 'plan_id UUID REFERENCES mealplans.plans (id)'
expect_pass 'downstream reads and foreign keys'

clear_case
put apps/recipes/internal/repositories/case.go 'const q = `SELECT 1 FROM mealplans.plans`'
expect_fail 'upstream reading downstream' 'apps/recipes/internal/repositories/case.go:1: recipes may not read mealplans.plans'

clear_case
put apps/books/internal/repositories/case.go 'const q = `SELECT 1 FROM feeds.items`'
expect_fail 'sideways read' 'books may not read feeds.items'

clear_case
put "$plans" 'const q = `SELECT 1 FROM shoppinglist.custom_items`'
expect_fail 'mealplans reading shoppinglist' 'mealplans may not read shoppinglist.custom_items'

clear_case
put apps/recipes/migrations/case.sql 'plan_id UUID REFERENCES mealplans.plans (id)'
expect_fail 'upstream foreign key' 'recipes may not read mealplans.plans'

clear_case
put "$plans" 'const q = `
  SELECT 1
  FROM
    shoppinglist.custom_items`'
expect_fail 'reference split across lines' 'case.go:3: mealplans may not read shoppinglist.custom_items'

clear_case
put "$plans" 'const q = `INSERT INTO recipes.recipes (id) VALUES (1)`'
expect_fail 'unlisted write into an upstream schema' 'mealplans may not write recipes.recipes'

clear_case
put "$repo" 'const q = `DELETE FROM mealplans.plans WHERE true`'
expect_fail 'unlisted write by a downstream app' 'shoppinglist may not write mealplans.plans'

clear_case
put "$repo" 'const q = `UPDATE mealplans.plan_meals pm SET x = 1`'
expect_pass 'listed write'

clear_case
put apps/mealplans/migrations/case.sql 'ALTER TABLE recipes.plans SET SCHEMA mealplans;'
expect_pass 'listed migration adoption'

clear_case
put apps/books/internal/repositories/case_test.go 'const q = `INSERT INTO feeds.feeds (id) VALUES (1)`'
expect_pass 'test files are exempt'

clear_case
put apps/books/internal/repositories/case.go '// reads feeds.items for the cover
const q = `SELECT 1`'
put apps/books/migrations/case.sql '-- INSERT INTO feeds.feeds is documented, not run'
expect_pass 'comments are ignored'

clear_case
put apps/books/internal/repositories/case.go 'const q = `SELECT 1 FROM global.families JOIN books.books ON true`'
expect_pass 'own and shared schemas are not cross-schema'

if [ "$fail_count" -ne 0 ]; then
  printf '\n%d case(s) failed\n' "$fail_count"
  exit 1
fi
