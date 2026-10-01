#!/usr/bin/env bash
# File or update a tracking issue when the scheduled health check fails. Called
# by .github/workflows/health-check.yml's report job after any lint/test job
# fails. Detection-only: it reads state and writes an issue, never code.
#
# Inputs: GH_TOKEN, GITHUB_REPOSITORY, HEALTH_CHECK_NEEDS (JSON of the caller's
# `needs`), HEALTH_CHECK_RUN_ID, HEALTH_CHECK_RUN_URL; HEALTH_CHECK_TITLE optional.
set -euo pipefail

: "${GH_TOKEN:?GH_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${HEALTH_CHECK_NEEDS:?HEALTH_CHECK_NEEDS is required}"
: "${HEALTH_CHECK_RUN_ID:?HEALTH_CHECK_RUN_ID is required}"
: "${HEALTH_CHECK_RUN_URL:?HEALTH_CHECK_RUN_URL is required}"

title="${HEALTH_CHECK_TITLE:-Scheduled health check is failing}"
max_log_lines=200

failed_jobs=$(
	printf '%s' "$HEALTH_CHECK_NEEDS" |
		jq -r 'to_entries[] | select(.value.result == "failure" or .value.result == "cancelled") | .key' |
		sort | paste -sd, - | sed 's/,/, /g'
)
if [ -z "$failed_jobs" ]; then
	echo "No failed jobs reported; nothing to file."
	exit 0
fi

# Bounded excerpt of the failed-job logs; the run link carries the full output.
log_excerpt=$(
	gh run view "$HEALTH_CHECK_RUN_ID" --repo "$GITHUB_REPOSITORY" --log-failed 2>/dev/null |
		tail -n "$max_log_lines" || true
)

body_file=$(mktemp)
trap 'rm -f "$body_file"' EXIT
{
	echo "The scheduled repository health check failed."
	echo
	echo "- Failed jobs: ${failed_jobs}"
	echo "- Run: ${HEALTH_CHECK_RUN_URL}"
	echo "- Detected: $(date -u +%Y-%m-%dT%H:%MZ) (UTC)"
	echo
	echo "Opened automatically by \`.github/workflows/health-check.yml\`."
	echo "It never pushes code, so fixing this is a normal agent or human task."
	if [ -n "$log_excerpt" ]; then
		echo
		echo '```'
		printf '%s\n' "$log_excerpt"
		echo '```'
	fi
} >"$body_file"

existing=$(
	gh issue list --repo "$GITHUB_REPOSITORY" --state open --limit 100 --json number,title |
		jq -r --arg t "$title" '.[] | select(.title == $t) | .number' | head -n 1
)
if [ -n "$existing" ]; then
	gh issue comment "$existing" --repo "$GITHUB_REPOSITORY" --body-file "$body_file"
	echo "Commented on existing issue #$existing"
else
	gh issue create --repo "$GITHUB_REPOSITORY" --title "$title" \
		--label ci --label bug --body-file "$body_file"
fi
