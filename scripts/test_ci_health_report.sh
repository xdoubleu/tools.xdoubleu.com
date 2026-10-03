#!/usr/bin/env bash
# Tests ci_health_report.sh's create/comment/no-op branches with a stubbed gh.
set -euo pipefail

cd "$(dirname "$0")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() {
	echo "FAIL: $1" >&2
	echo "  want: $2" >&2
	echo "  got:  $3" >&2
	exit 1
}

check_contains() {
	local name=$1 needle=$2 haystack=$3
	case "$haystack" in
	*"$needle"*) ;;
	*) fail "$name" "contains: $needle" "$haystack" ;;
	esac
}

export GH_TOKEN=test GITHUB_REPOSITORY=owner/repo
export HEALTH_CHECK_RUN_ID=42 HEALTH_CHECK_RUN_URL=https://example/run/42
export FAKE_CALLS="$tmp/calls" FAKE_JOBS="$tmp/jobs" FAKE_JOB_LOGS="$tmp/joblogs"
export FAKE_ISSUE_LIST="$tmp/issues" FAKE_BODY_OUT="$tmp/body"

write_stub() {
	mkdir -p "$tmp/bin"
	cat >"$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_CALLS"
case "$*" in
*"actions/jobs/"*"/logs"*) cat "$FAKE_JOB_LOGS" ;;
*"actions/runs/"*"jobs"*) cat "$FAKE_JOBS" ;;
*"issue list"*) cat "$FAKE_ISSUE_LIST" ;;
*"issue create"* | *"issue comment"*)
	body=""
	while [ $# -gt 0 ]; do
		[ "$1" = "--body-file" ] && body=$2
		shift
	done
	[ -n "$body" ] && cat "$body" >"$FAKE_BODY_OUT"
	;;
esac
STUB
	chmod +x "$tmp/bin/gh"
	export PATH="$tmp/bin:$PATH"
}
write_stub

printf '111\tForced failure\n' >"$FAKE_JOBS"
echo "boom: lint failed" >"$FAKE_JOB_LOGS"

run() {
	: >"$FAKE_CALLS"
	: >"$FAKE_BODY_OUT"
	./ci_health_report.sh
}

# All green: no gh issue write at all.
echo '[]' >"$FAKE_ISSUE_LIST"
: >"$FAKE_JOBS"
run
case "$(cat "$FAKE_CALLS")" in
*"issue create"* | *"issue comment"*) fail "green no-op" "no issue write" "$(cat "$FAKE_CALLS")" ;;
esac

# Failure with no existing issue: files one, with the failing jobs and logs.
printf '111\tForced failure\n' >"$FAKE_JOBS"
run
check_contains "create called" "issue create" "$(cat "$FAKE_CALLS")"
check_contains "body lists job" "Forced failure" "$(cat "$FAKE_BODY_OUT")"
check_contains "body links run" "https://example/run/42" "$(cat "$FAKE_BODY_OUT")"
check_contains "body has job logs" "boom: lint failed" "$(cat "$FAKE_BODY_OUT")"

# An open matching issue exists: comment instead of creating a duplicate.
echo '[{"number":123,"title":"Scheduled health check is failing"}]' >"$FAKE_ISSUE_LIST"
run
check_contains "comment called" "issue comment 123" "$(cat "$FAKE_CALLS")"
case "$(cat "$FAKE_CALLS")" in
*"issue create"*) fail "dedupe" "no create" "$(cat "$FAKE_CALLS")" ;;
esac

# Log fetch failing must not stop the issue from being filed.
echo '[]' >"$FAKE_ISSUE_LIST"
cat >"$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_CALLS"
case "$*" in
*"/logs"*) exit 1 ;;
*"actions/runs/"*"jobs"*) printf '111\tForced failure\n' ;;
*"issue list"*) echo '[]' ;;
*"issue create"*)
	body=""
	while [ $# -gt 0 ]; do
		[ "$1" = "--body-file" ] && body=$2
		shift
	done
	[ -n "$body" ] && cat "$body" >"$FAKE_BODY_OUT"
	;;
esac
STUB
chmod +x "$tmp/bin/gh"
run
check_contains "create despite log error" "issue create" "$(cat "$FAKE_CALLS")"
check_contains "body still lists job" "Forced failure" "$(cat "$FAKE_BODY_OUT")"

echo "ci_health_report.sh: all tests passed"
