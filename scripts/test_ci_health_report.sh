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

# Stub gh: dispatch on the subcommand, record calls, keep the issue body.
mkdir -p "$tmp/bin"
cat >"$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_CALLS"
case "$1 $2" in
"run view") cat "$FAKE_LOGS" ;;
"issue list") cat "$FAKE_ISSUE_LIST" ;;
"issue create" | "issue comment")
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

export GH_TOKEN=test GITHUB_REPOSITORY=owner/repo
export HEALTH_CHECK_RUN_ID=42 HEALTH_CHECK_RUN_URL=https://example/run/42
export FAKE_CALLS="$tmp/calls" FAKE_LOGS="$tmp/logs" FAKE_ISSUE_LIST="$tmp/issues" FAKE_BODY_OUT="$tmp/body"
echo "boom: lint failed" >"$FAKE_LOGS"

run() {
	: >"$FAKE_CALLS"
	: >"$FAKE_BODY_OUT"
	HEALTH_CHECK_NEEDS=$1 ./ci_health_report.sh
}

# All green: no gh write call at all.
echo '[]' >"$FAKE_ISSUE_LIST"
run '{"api-lint":{"result":"success"},"report":{"result":"skipped"}}'
[ -s "$FAKE_CALLS" ] && fail "green no-op" "no gh calls" "$(cat "$FAKE_CALLS")"

# Failure with no existing issue: files one, with the failing jobs and logs.
run '{"api-lint":{"result":"failure"},"web-test":{"result":"success"}}'
check_contains "create called" "issue create" "$(cat "$FAKE_CALLS")"
check_contains "body lists job" "api-lint" "$(cat "$FAKE_BODY_OUT")"
check_contains "body links run" "https://example/run/42" "$(cat "$FAKE_BODY_OUT")"
check_contains "body has logs" "boom: lint failed" "$(cat "$FAKE_BODY_OUT")"

# An open matching issue exists: comment instead of creating a duplicate.
echo '[{"number":123,"title":"Scheduled health check is failing"}]' >"$FAKE_ISSUE_LIST"
run '{"api-lint":{"result":"failure"}}'
check_contains "comment called" "issue comment 123" "$(cat "$FAKE_CALLS")"
case "$(cat "$FAKE_CALLS")" in
*"issue create"*) fail "dedupe" "no create" "$(cat "$FAKE_CALLS")" ;;
esac

# Log fetch failing must not stop the issue from being filed.
echo '[]' >"$FAKE_ISSUE_LIST"
cat >"$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_CALLS"
case "$1 $2" in
"run view") exit 1 ;;
"issue list") echo '[]' ;;
"issue create")
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
run '{"api-lint":{"result":"failure"}}'
check_contains "create despite log error" "issue create" "$(cat "$FAKE_CALLS")"
check_contains "body still lists job" "api-lint" "$(cat "$FAKE_BODY_OUT")"

echo "ci_health_report.sh: all tests passed"
