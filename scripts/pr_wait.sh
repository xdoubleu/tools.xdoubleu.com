#!/usr/bin/env bash
# Waits for a PR's CI checks, then for its terminal state, as plain commands a
# worktree-isolated session runs without a "too complex to verify" refusal.
# Usage: pr_wait.sh <pr> [merged|mergeable]   (default merged: auto-merge armed)
# Exit: 0 merged/mergeable, 1 a check failed, 2 closed unmerged or conflicting.
set -uo pipefail

pr=${1:?usage: pr_wait.sh <pr> [merged|mergeable]}
mode=${2:-merged}
interval=${PR_WAIT_INTERVAL:-20}

while :; do
  out=$(gh pr checks "$pr" --watch --fail-fast 2>&1 | tail -60)
  code=${PIPESTATUS[0]}
  # gh reports no checks until GitHub registers the push; transport errors are
  # not check results. Only a real check result ends the watch.
  if grep -qE "no checks reported|connection reset by peer|EOF|HTTP 5|internal server error" <<<"$out"; then
    sleep "$interval"
    continue
  fi
  echo "$out"
  [[ "$code" -eq 0 ]] || exit 1
  break
done

while :; do
  line=$(gh pr view "$pr" --json state,mergeable,mergeStateStatus \
    --jq '"\(.state) \(.mergeable) \(.mergeStateStatus)"' 2>/dev/null) || line=""
  echo "state/mergeable/mergeStateStatus: ${line:-unknown}"
  read -r state mergeable mergestate <<<"$line"
  case "${state:-}" in
    MERGED) exit 0 ;;
    CLOSED) exit 2 ;;
  esac
  [[ "${mergestate:-}" != DIRTY ]] || exit 2
  if [[ "$mode" == mergeable && "${mergeable:-}" == MERGEABLE ]]; then
    exit 0
  fi
  sleep "$interval"
done
