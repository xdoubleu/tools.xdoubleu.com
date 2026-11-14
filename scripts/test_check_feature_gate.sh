#!/usr/bin/env bash
# Exercises scripts/check_feature_gate.sh.
set -uo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fails=0

# check NAME EXPECTED_EXIT LABELS BODY
check() {
  PR_LABELS="$3" PR_BODY="$4" "$ROOT_DIR/scripts/check_feature_gate.sh" >/dev/null 2>&1
  local got=$?
  if [[ $got -eq $2 ]]; then echo "PASS: $1"; else echo "FAIL: $1 (exit $got)"; fails=$((fails + 1)); fi
}

check "non-feature needs nothing" 0 $'bug\nchore' "no evidence"
check "feature with evidence passes" 0 $'feature' $'did it\n\nFeature gate: mutation=pass, review=pass'
check "feature without line fails" 1 $'feature' "did it"
check "skipped mutation fails" 1 $'feature' "Feature gate: mutation=skipped, review=pass"
check "missing review fails" 1 $'feature' "Feature gate: mutation=pass"
check "label match is exact" 0 $'feature-flag' "x"

exit $((fails > 0))
