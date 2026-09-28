#!/usr/bin/env bash
# Resumes the agent's OpenCode session to run its closing steps when it ended
# without a valid outcome file or a summary (it quit early or hit its cap):
#   routine_finish.sh TRANSCRIPT LOG OUTCOME_PATH SUMMARY_PATH MODEL PROMPT
# Appends to TRANSCRIPT and LOG. Never fails the step: a failed finish still
# leaves the workflow's own fallbacks. OPENCODE and ROUTINE_FINISH_TIMEOUT
# override the binary and the cap.
set -uo pipefail

transcript=${1:?transcript} log=${2:?log} outcome=${3:?outcome path}
summary=${4:?summary path} model=${5:?model} prompt=${6:?prompt}

if jq -e '.outcome | IN("succeeded", "failed", "no_action_needed")' \
  "$outcome" >/dev/null 2>&1 && [ -s "$summary" ]; then
  exit 0
fi

session=$(jq -rR 'fromjson? | .sessionID? // empty | select(length > 0)' \
  "$transcript" 2>/dev/null | head -n 1)
if [ -z "$session" ]; then
  echo "::warning::Agent wrote no outcome or summary and left no session to resume."
  exit 0
fi

echo "Agent wrote no outcome or summary; resuming its session to finish."
timeout "${ROUTINE_FINISH_TIMEOUT:-6m}" "${OPENCODE:-opencode}" run --standalone \
  --auto --model "openrouter/${model}" --session "$session" --format json \
  "$prompt" >>"$transcript" 2>>"$log"
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "::warning::Finishing run exited $rc."
fi
exit 0
