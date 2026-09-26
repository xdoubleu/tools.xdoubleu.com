#!/usr/bin/env bash
# Tests routine_metrics.sh against a fixture transcript.
set -euo pipefail

cd "$(dirname "$0")"
fixture=testdata/routine_transcript.jsonl
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() {
  echo "FAIL: $1" >&2
  echo "  want: $2" >&2
  echo "  got:  $3" >&2
  exit 1
}

check() {
  local name=$1 want=$2 got=$3
  [ "$got" = "$want" ] || fail "$name" "$want" "$got"
}

check "json" \
  '{"requests":3,"input_tokens":3300,"output_tokens":70,"reasoning_tokens":90,"cache_read_tokens":600,"cost_usd":0.00035,"duration_seconds":10.5,"tool_calls":5,"tool_errors":2,"repeated_tool_calls":1}' \
  "$(./routine_metrics.sh json "$fixture")"

check "line" \
  '3 requests, 3370 tokens (90 reasoning), ~$0.0004, 11s, 5 tool calls (2 failed, 1 repeated)' \
  "$(./routine_metrics.sh line "$fixture")"

check "markdown tool table" \
  '| bash | 3 |' \
  "$(./routine_metrics.sh markdown "$fixture" | sed -n '/| Tool |/,$p' | sed -n 3p)"

# A run killed mid-write leaves a truncated last line.
{ cat "$fixture"; printf '{"type":"step_finish","part":{"tok'; } >"$tmp/cut.jsonl"
check "truncated transcript" \
  "$(./routine_metrics.sh json "$fixture")" \
  "$(./routine_metrics.sh json "$tmp/cut.jsonl")"

: >"$tmp/empty.jsonl"
check "empty transcript" \
  '{"requests":0,"input_tokens":0,"output_tokens":0,"reasoning_tokens":0,"cache_read_tokens":0,"cost_usd":0,"duration_seconds":0,"tool_calls":0,"tool_errors":0,"repeated_tool_calls":0}' \
  "$(./routine_metrics.sh json "$tmp/empty.jsonl")"

if ./routine_metrics.sh bogus "$fixture" 2>/dev/null; then
  fail "unknown mode" "exit 2" "exit 0"
fi

echo "routine_metrics.sh: all tests passed"
