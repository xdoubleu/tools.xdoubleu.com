#!/usr/bin/env bash
# File or update a tracking issue when a scheduled health-check run failed,
# called by .github/workflows/health-check-report.yml once the run finishes so
# per-job logs can be quoted. Detection-only: writes an issue, never code.
#
# Inputs: GH_TOKEN, GITHUB_REPOSITORY, HEALTH_CHECK_RUN_ID, HEALTH_CHECK_RUN_URL;
# HEALTH_CHECK_TITLE optional.
set -euo pipefail

: "${GH_TOKEN:?GH_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${HEALTH_CHECK_RUN_ID:?HEALTH_CHECK_RUN_ID is required}"
: "${HEALTH_CHECK_RUN_URL:?HEALTH_CHECK_RUN_URL is required}"

title="${HEALTH_CHECK_TITLE:-Scheduled health check is failing}"
max_log_lines=200
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

# Failed jobs come from the API, not the caller, so the report works whether it
# runs inside the failing run or (as now) in a separate workflow_run.
jobs_file="$tmp_dir/jobs"
gh api --paginate "repos/$GITHUB_REPOSITORY/actions/runs/$HEALTH_CHECK_RUN_ID/jobs?per_page=100" \
	--jq '.jobs[] | select(.conclusion == "failure" or .conclusion == "cancelled" or .conclusion == "timed_out") | "\(.id)\t\(.name)"' \
	>"$jobs_file" 2>/dev/null || true
if [ ! -s "$jobs_file" ]; then
	echo "No failed jobs reported; nothing to file."
	exit 0
fi
failed_jobs=$(cut -f2 "$jobs_file" | sort | paste -sd, - | sed 's/,/, /g')

# Bounded excerpt of each failed job's log; the run link carries the full output.
log_all=""
while IFS=$'\t' read -r job_id job_name; do
	[ -n "$job_id" ] || continue
	job_log=$(gh api "repos/$GITHUB_REPOSITORY/actions/jobs/$job_id/logs" 2>/dev/null || true)
	log_all="${log_all}== ${job_name} ==
${job_log}
"
done <"$jobs_file"
log_excerpt=$(printf '%s' "$log_all" | tail -n "$max_log_lines")

body_file="$tmp_dir/body"
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
