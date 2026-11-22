#!/usr/bin/env bash
# Exercises check_hexagonal.sh against a synthetic api/ tree.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check_hexagonal.sh"

fail_count=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n%s\n' "$1" "$2"; fail_count=$((fail_count + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

api="$WORK/api"
mkdir -p "$api/apps/demo/connect" "$api/apps/demo/mcp" "$api/scripts"
cp "$SCRIPT" "$api/scripts/"
imp='import "tools.xdoubleu.com/apps/demo/internal/repositories"'
printf 'package demo\n%s\n' "$imp" > "$api/apps/demo/app.go"
printf 'package connect\n' > "$api/apps/demo/connect/ok.go"
printf 'package connect\n%s\n' "$imp" > "$api/apps/demo/connect/ok_test.go"

out=$("$api/scripts/check_hexagonal.sh" 2>&1) \
  && pass "app.go and tests may import repositories" \
  || fail "clean tree rejected" "$out"

check_fails() {
  printf 'package x\n%s\n' "$imp" > "$api/$2"
  out=$("$api/scripts/check_hexagonal.sh" 2>&1) \
    && fail "$1" "expected failure" \
    || { grep -q "$2" <<<"$out" && pass "$1" || fail "$1" "$out"; }
  rm "$api/$2"
}
check_fails "connect import is rejected" apps/demo/connect/bad.go
check_fails "mcp import is rejected" apps/demo/mcp/bad.go
check_fails "root package import outside app.go is rejected" apps/demo/routes.go

exit $((fail_count > 0))
