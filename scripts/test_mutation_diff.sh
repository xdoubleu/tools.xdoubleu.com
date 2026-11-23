#!/usr/bin/env bash
# Runs the mutation-diff helper scripts against a scratch repo with changed Go
# and TS files, so a broken import or a wrong path fails here, not unnoticed.
set -uo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fails=0

check() {
  if [[ "$2" == "$3" ]]; then echo "PASS: $1"; else echo "FAIL: $1: got '$2', want '$3'"; fails=$((fails + 1)); fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cd "$tmp" || exit 1
git init -q -b main . && git config user.email t@t && git config user.name t
mkdir -p api/pkg/a web/lib
printf 'package a\n\nfunc F() int {\n\treturn 1\n}\n' >api/pkg/a/a.go
printf 'export const f = 1\nexport const g = 2\n' >web/lib/x.ts
git add -A && git commit -qm base
git update-ref refs/remotes/origin/main HEAD
git switch -qc work
printf 'package a\n\nfunc F() int {\n\treturn 2\n}\n' >api/pkg/a/a.go
printf 'export const f = 1\nexport const g = 3\n' >web/lib/x.ts
printf 'package b\n' >api/pkg/b.go
git add -A && git commit -qm change

check "go packages" "$(cd api && python3 "$ROOT_DIR/tools/diff_packages_go.py" 2>&1)" ./pkg
check "ts entries" "$(cd web && python3 "$ROOT_DIR/tools/diff_files_ts.py" 2>&1)" "lib/x.ts:2-2"
printf '{"files":[]}' >"$tmp/r.json"
(cd api && python3 "$ROOT_DIR/tools/check_mutation_report_go.py" ./pkg/a "$tmp/r.json" >/dev/null 2>&1)
check "report checker imports" "$?" "0"

# A failing helper must fail the make target, not read as "nothing changed".
mkdir "$tmp/bin" && printf '#!/bin/sh\nexit 1\n' >"$tmp/bin/python3" && chmod +x "$tmp/bin/python3"
for dir in api factory-runner; do
  PATH="$tmp/bin:$PATH" make -s -C "$ROOT_DIR/$dir" -o tools/mutation test/mutation/diff >/dev/null 2>&1
  check "$dir target fails closed" "$?" "2"
done
PATH="$tmp/bin:$PATH" bash "$ROOT_DIR/web/scripts/test-mutation-diff.sh" >/dev/null 2>&1
check "web script fails closed" "$?" "1"

exit $((fails > 0))
