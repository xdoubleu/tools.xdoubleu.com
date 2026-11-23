#!/usr/bin/env bash
# Exercises scripts/fetch_doc_budgets.sh's proxy path with a file:// "proxy".
set -uo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
fails=0
pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; fails=$((fails + 1)); }

rev=0123abc
src="$work/src/skills-$rev/plugins/doc-budgets/scripts"
mkdir -p "$src" "$work/proxy/nixsrc/xdoubleu/skills/tar.gz"
printf '#!/bin/sh\necho budgets ok\n' >"$src/lint_docs.sh"
chmod +x "$src/lint_docs.sh"
tar -czf "$work/proxy/nixsrc/xdoubleu/skills/tar.gz/$rev" -C "$work/src" "skills-$rev"

dir="$work/cache/$rev"
if WARDCASE_PROXY="file://$work/proxy" "$ROOT_DIR/scripts/fetch_doc_budgets.sh" "$rev" "$dir" &&
  [[ "$("$dir/scripts/lint_docs.sh")" == "budgets ok" ]] && [[ ! -e "$dir.tmp" ]]; then
  pass "a job fetches the plugin through the proxy's nixsrc route"
else
  fail "a job fetches the plugin through the proxy's nixsrc route"
fi

dir="$work/cache/missing"
if WARDCASE_PROXY="file://$work/proxy" "$ROOT_DIR/scripts/fetch_doc_budgets.sh" missing "$dir" 2>/dev/null; then
  fail "an unknown revision fails"
elif [[ -e "$dir" || -e "$dir.tmp" ]]; then
  fail "a failed fetch leaves nothing behind"
else
  pass "an unknown revision fails and leaves nothing behind"
fi

exit $((fails > 0))
