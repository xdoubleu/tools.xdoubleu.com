#!/usr/bin/env bash
# Exercises scripts/routine_watchdog.sh against synthetic transcripts: it must
# trip on a degenerate identical-tool loop and stay quiet on varied work.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WATCHDOG="$ROOT_DIR/scripts/routine_watchdog.sh"

fail=0
pass() { printf 'PASS: %s\n' "$1"; }
failn() { printf 'FAIL: %s\n' "$1"; fail=1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# degenerate: 40 identical `bash: true` calls
python3 - "$WORK/degen.jsonl" <<'PY'
import json, sys
ts = 1790000000000
def ev(cmd):
    global ts; ts += 500
    return json.dumps({"type": "tool_use", "timestamp": ts, "sessionID": "s",
        "part": {"type": "tool", "tool": "bash", "callID": "c",
                 "state": {"status": "completed", "input": {"command": cmd}}}})
f = open(sys.argv[1], "w")
f.write(json.dumps({"type": "step_start", "timestamp": ts, "sessionID": "s", "part": {}}) + "\n")
for _ in range(40):
    f.write(ev("true") + "\n")
f.close()
PY

# benign: distinct commands per step, no repeats
python3 - "$WORK/benign.jsonl" <<'PY'
import json, sys
ts = 1790000000000
def ev(cmd):
    global ts; ts += 500
    return json.dumps({"type": "tool_use", "timestamp": ts, "sessionID": "s",
        "part": {"type": "tool", "tool": "bash", "callID": "c",
                 "state": {"status": "completed", "input": {"command": cmd}}}})
def st(t):
    global ts; ts += 500
    return json.dumps({"type": t, "timestamp": ts, "sessionID": "s", "part": {}})
f = open(sys.argv[1], "w")
for i in range(100):
    f.write(st("step_start") + "\n")
    for j in range(3):
        f.write(ev("echo %d_%d" % (i, j)) + "\n")
    f.write(st("step_finish") + "\n")
f.close()
PY

# stale stamp to prove each run clears/recreates it
run_watchdog() {
  local file="$1" name="$2"
  local stamp="$WORK/$name.stamp"
  rm -f "$stamp"
  ( : > "$WORK/pid" )
  "$WATCHDOG" --transcript "$file" --stamp "$stamp" --kill-pid 999999 \
    --window 30 --poll 1 > "$WORK/$name.out" 2>&1 &
  local wp=$!
  rm -f "$WORK/$name.done"
  for _ in $(seq 1 20); do
    [ -f "$stamp" ] && break
    sleep 0.1
  done
  kill "$wp" 2>/dev/null || true
  wait "$wp" 2>/dev/null || true
  [ -f "$stamp" ] && echo trip || echo quiet
}

bash -n "$WATCHDOG" || { failn "routine_watchdog.sh bash -n"; exit 1; }
pass "routine_watchdog.sh parses"

if [ "$(run_watchdog "$WORK/degen.jsonl" degen)" = "trip" ]; then
  pass "degenerate loop trips the watchdog"
else
  failn "degenerate loop did not trip"
fi

if [ "$(run_watchdog "$WORK/benign.jsonl" benign)" = "quiet" ]; then
  pass "varied benign work stays quiet"
else
  failn "varied benign work false-tripped"
fi

exit "$fail"