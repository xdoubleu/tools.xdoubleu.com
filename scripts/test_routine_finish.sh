#!/usr/bin/env bash
# Exercises scripts/routine_finish.sh against a fake opencode: it must resume
# the transcript's session only when the outcome or summary is missing, and
# never fail the step.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FINISH="$ROOT_DIR/scripts/routine_finish.sh"

fail=0
pass() { printf 'PASS: %s\n' "$1"; }
failn() { printf 'FAIL: %s\n' "$1"; fail=1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# Records its arguments and, like a well-behaved agent, writes both files.
cat > "$WORK/opencode" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$FAKE_ARGS"
[ -n "${FAKE_RC:-}" ] && exit "$FAKE_RC"
echo '{"outcome": "succeeded"}' > "$FAKE_OUTCOME"
echo 'summary' > "$FAKE_SUMMARY"
echo '{"type":"step_finish","sessionID":"ses_1"}'
SH
chmod +x "$WORK/opencode"

export OPENCODE="$WORK/opencode" FAKE_ARGS="$WORK/args"
export FAKE_OUTCOME="$WORK/outcome.json" FAKE_SUMMARY="$WORK/summary.md"

# setup TRANSCRIPT_LINES OUTCOME SUMMARY resets the run's files.
setup() {
  rm -f "$FAKE_ARGS" "$FAKE_OUTCOME" "$FAKE_SUMMARY"
  printf '%s' "$1" > "$WORK/transcript.jsonl"
  : > "$WORK/opencode.log"
  if [ -n "$2" ]; then echo "$2" > "$FAKE_OUTCOME"; fi
  if [ -n "$3" ]; then echo "$3" > "$FAKE_SUMMARY"; fi
}

run() {
  "$FINISH" "$WORK/transcript.jsonl" "$WORK/opencode.log" "$FAKE_OUTCOME" \
    "$FAKE_SUMMARY" some/model 'time is up' > "$WORK/out" 2>&1
}

TRANSCRIPT='not json
{"type":"error","sessionID":""}
{"type":"step_start","sessionID":"ses_1"}
{"type":"step_start","sessionID":"ses_1"}
'

setup "$TRANSCRIPT" '{"outcome": "no_action_needed"}' 'done'
if run && [ ! -e "$FAKE_ARGS" ]; then
  pass "outcome and summary present: no resume"
else
  failn "outcome and summary present: no resume"
fi

setup "$TRANSCRIPT" '' ''
if run && [ "$(tr '\n' ' ' < "$FAKE_ARGS")" = \
  "run --standalone --auto --model openrouter/some/model --session ses_1 --format json time is up " ]; then
  pass "nothing written: resumes the transcript's first session"
else
  failn "nothing written: resumes the transcript's first session ($(cat "$FAKE_ARGS" 2>/dev/null))"
fi
if [ -s "$FAKE_SUMMARY" ] && [ "$(wc -l < "$WORK/transcript.jsonl")" -eq 5 ]; then
  pass "resume appends to the transcript"
else
  failn "resume appends to the transcript"
fi

setup "$TRANSCRIPT" '{"outcome": "succeeded"}' ''
if run && [ -e "$FAKE_ARGS" ]; then
  pass "summary missing: resumes"
else
  failn "summary missing: resumes"
fi

setup "$TRANSCRIPT" '{"outcome": "done"}' 'done'
if run && [ -e "$FAKE_ARGS" ]; then
  pass "invalid outcome: resumes"
else
  failn "invalid outcome: resumes"
fi

setup '{"type":"error","sessionID":""}' '' ''
if run && [ ! -e "$FAKE_ARGS" ] && grep -q 'no session to resume' "$WORK/out"; then
  pass "no session: warns, doesn't resume"
else
  failn "no session: warns, doesn't resume"
fi

setup "$TRANSCRIPT" '' ''
if FAKE_RC=3 run && grep -q 'Finishing run exited 3' "$WORK/out"; then
  pass "failed resume: warns, exits 0"
else
  failn "failed resume: warns, exits 0"
fi

setup "$TRANSCRIPT" '' ''
printf '#!/usr/bin/env bash\nsleep 5\n' > "$WORK/slow"
chmod +x "$WORK/slow"
if OPENCODE="$WORK/slow" ROUTINE_FINISH_TIMEOUT=1s run \
  && grep -q 'Finishing run exited 124' "$WORK/out"; then
  pass "resume is capped by ROUTINE_FINISH_TIMEOUT"
else
  failn "resume is capped by ROUTINE_FINISH_TIMEOUT"
fi

exit "$fail"
