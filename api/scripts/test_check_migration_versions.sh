#!/usr/bin/env bash
# Exercises check_migration_versions.sh against a synthetic git repo.
set -uo pipefail

SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/check_migration_versions.sh"

fail_count=0
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n%s\n' "$1" "$2"; fail_count=$((fail_count + 1)); }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

api="$WORK/api"
dir="$api/apps/demo/migrations"
mkdir -p "$dir" "$api/scripts"
cp "$SCRIPT" "$api/scripts/"
git -C "$WORK" init -q -b main
git -C "$WORK" config user.email test@example.com
git -C "$WORK" config user.name test
touch "$dir/00001_init.sql"
git -C "$WORK" add -A
git -C "$WORK" commit -q -m base

run() { BASE_REF=main "$api/scripts/check_migration_versions.sh" 2>&1; }

expect() { # name, want-exit, want-substring
	local out code
	out=$(run)
	code=$?
	if [ "$code" -eq "$2" ] && { [ -z "$3" ] || printf '%s' "$out" | grep -qF -- "$3"; }; then
		pass "$1"
	else
		fail "$1" "exit $code: $out"
	fi
}

git -C "$WORK" checkout -q -b feature
expect "existing sequential migration passes" 0 ""

touch "$dir/20261118090000_ok.sql"
expect "untracked timestamp migration passes" 0 ""

touch "$dir/00002_untracked.sql"
expect "untracked sequential migration fails" 1 "00002_untracked.sql"

git -C "$WORK" add "$dir/00002_untracked.sql"
expect "staged sequential migration fails" 1 "00002_untracked.sql"

git -C "$WORK" commit -q -m add
expect "committed sequential migration fails" 1 "00002_untracked.sql"

git -C "$WORK" rm -q "$dir/00002_untracked.sql"
cp "$dir/00001_init.sql" "$dir/00001_dupe.sql"
expect "duplicate version fails" 1 "duplicate migration version 00001"

if [ "$fail_count" -ne 0 ]; then
	echo "$fail_count test(s) failed"
	exit 1
fi
echo "all tests passed"
