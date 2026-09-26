#!/usr/bin/env bash
# Summarizes an OpenCode `--format json` transcript as run metrics: counts
# only, never content, since the output lands in public job logs.
#   routine_metrics.sh json TRANSCRIPT      record_action's metrics object
#   routine_metrics.sh markdown TRANSCRIPT  a table for $GITHUB_STEP_SUMMARY
#   routine_metrics.sh line TRANSCRIPT      one line for Slack
set -euo pipefail

mode=${1:?usage: routine_metrics.sh json|markdown|line TRANSCRIPT}
transcript=${2:?usage: routine_metrics.sh json|markdown|line TRANSCRIPT}

# A tool call failed when OpenCode says so or bash exited non-zero. A repeat
# is a call identical in tool and input to the call just before it. Lines
# that aren't JSON are skipped: a timed-out run can end mid-line.
metrics=$(jq -Rn '
  [inputs | fromjson? | select(type == "object")] as $events
  | [$events[] | select(.type == "step_finish") | .part] as $steps
  | [$events[] | select(.type == "tool_use") | .part] as $tools
  | [$events[] | .timestamp | numbers] as $ts
  | {
      requests: ($steps | length),
      input_tokens: ([$steps[].tokens.input // 0] | add // 0),
      output_tokens: ([$steps[].tokens.output // 0] | add // 0),
      reasoning_tokens: ([$steps[].tokens.reasoning // 0] | add // 0),
      cache_read_tokens: ([$steps[].tokens.cache.read // 0] | add // 0),
      cost_usd: ([$steps[].cost // 0] | add // 0 | . * 1e6 | round / 1e6),
      duration_seconds:
        (if ($ts | length) > 1 then (($ts | max) - ($ts | min)) / 1000 else 0 end),
      tool_calls: ($tools | length),
      tool_errors: ([$tools[] | select(.state.status == "error"
        or ((.state.metadata.exit // 0) != 0))] | length),
      repeated_tool_calls: ([range(1; $tools | length)
        | select([$tools[.].tool, $tools[.].state.input]
            == [$tools[. - 1].tool, $tools[. - 1].state.input])] | length),
      tools_by_name: ($tools | group_by(.tool)
        | map({key: .[0].tool, value: length}) | from_entries)
    }
' "$transcript")

case $mode in
  json)
    jq -c 'del(.tools_by_name)' <<<"$metrics"
    ;;
  markdown)
    jq -r '
      "| Metric | Value |", "|---|---|",
      (to_entries[] | select(.key != "tools_by_name")
        | "| \(.key) | \(.value) |"),
      "", "| Tool | Calls |", "|---|---|",
      (.tools_by_name | to_entries | sort_by(-.value)[]
        | "| \(.key) | \(.value) |")
    ' <<<"$metrics"
    ;;
  line)
    jq -r '"\(.requests) requests, "
      + "\(.input_tokens + .output_tokens) tokens "
      + "(\(.reasoning_tokens) reasoning), "
      + "~$\(.cost_usd * 10000 | round / 10000), "
      + "\(.duration_seconds | round)s, "
      + "\(.tool_calls) tool calls (\(.tool_errors) failed, "
      + "\(.repeated_tool_calls) repeated)"' <<<"$metrics"
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 2
    ;;
esac
