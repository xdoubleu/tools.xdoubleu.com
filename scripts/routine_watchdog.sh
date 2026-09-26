#!/usr/bin/env bash
# Circuit-breaker for unattended OpenCode routine runs. Watches the live
# --format json transcript and kills the opencode process when the agent
# repeats the same tool call back to back without finishing a step (a
# degenerate loop, e.g. a bare `true`). Each call "succeeds", so nothing flags
# it and the job would spin until the step timeout. On trip it kills opencode
# and writes the stamp the workflow checks.

set -euo pipefail

transcript=""
stamp=""
kill_pid=""
window=${WATCHDOG_WINDOW:-30}
poll=${WATCHDOG_POLL:-2}

while [ "$#" -gt 0 ]; do
	case "$1" in
	--transcript) transcript="$2"; shift 2 ;;
	--stamp) stamp="$2"; shift 2 ;;
	--kill-pid) kill_pid="$2"; shift 2 ;;
	--window) window="$2"; shift 2 ;;
	--poll) poll="$2"; shift 2 ;;
	*) echo "unknown arg: $1" >&2; exit 2 ;;
	esac
done

[ -n "$transcript" ] && [ -n "$stamp" ] && [ -n "$kill_pid" ] \
	|| { echo "usage: --transcript <path> --stamp <path> --kill-pid <pid>" >&2; exit 2; }

processed=0   # number of transcript lines already handled
prev_key=""   # "tool\0command" of the last tool_use seen
run=0         # current run of identical tool_use

trip() {
	echo "routine_watchdog: degenerate tool loop detected; killing opencode ($kill_pid)" >&2
	kill "$kill_pid" 2>/dev/null || true
	: > "$stamp"
	exit 1
}

trap 'exit 0' TERM INT

while :; do
	# Re-read the whole (small) file each poll and handle only new lines.
	# A trailing line without a newline is still emitted by `read`.
	idx=0
	while IFS= read -r line; do
		if [ "$idx" -lt "$processed" ]; then
			idx=$((idx + 1))
			continue
		fi
		idx=$((idx + 1))
		processed=$idx
		[ -n "$line" ] || continue
		key=$(printf '%s' "$line" \
			| jq -r 'select(.type=="tool_use") | .part.tool + "\u0000" + ((.part.state.input.command // "") | tostring)' 2>/dev/null) \
			|| key=""
		[ -n "$key" ] || continue
		if [ "$key" = "$prev_key" ]; then
			run=$((run + 1))
			[ "$run" -ge "$window" ] && trip
		else
			prev_key="$key"
			run=1
		fi
	done < "$transcript"
	sleep "$poll"
done