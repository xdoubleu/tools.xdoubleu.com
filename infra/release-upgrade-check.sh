#!/usr/bin/env bash
# Checks locally on the VPS whether a new Ubuntu LTS release is available
# and, only then, emails a notification via Resend's HTTP API — run as root
# by release-upgrade-check.timer (issue #1194). Runs entirely on the box:
# nothing external SSHes in to perform or trigger this check, unlike the
# removed jobs.UbuntuReleaseJob (issue #1134), which polled Canonical's feed
# from the api process and compared it against a hardcoded baseline that had
# to be bumped by hand after every real upgrade — this script instead asks
# the box what it actually thinks, via do-release-upgrade -c, every run.
set -euo pipefail

ENV_FILE=/etc/release-upgrade-check.env
if [ -f "$ENV_FILE" ]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

OUTPUT="$(do-release-upgrade -c 2>&1 || true)"

if echo "$OUTPUT" | grep -q "There is no development version"; then
  echo "release-upgrade-check: no new LTS available"
  exit 0
fi

echo "release-upgrade-check: a new release appears to be available:"
echo "$OUTPUT"

if [ -z "${RESEND_API_KEY:-}" ] || [ -z "${NOTIFY_EMAIL_FROM:-}" ] || [ -z "${NOTIFY_EMAIL_TO:-}" ]; then
  echo "release-upgrade-check: RESEND_API_KEY/NOTIFY_EMAIL_FROM/NOTIFY_EMAIL_TO not set, skipping email" >&2
  exit 0
fi

HOSTNAME="$(hostname)"
SUBJECT="[Ubuntu] a new release is available on $HOSTNAME"
BODY="do-release-upgrade -c reported a new Ubuntu release is available on $HOSTNAME:

$OUTPUT"

# python3's json.dumps handles quoting/escaping of $OUTPUT (which may
# contain quotes, backslashes, newlines) correctly — hand-rolled sed/printf
# escaping is exactly the kind of thing that silently breaks Resend's JSON
# parse the first time do-release-upgrade's output contains a stray quote.
PAYLOAD="$(FROM="$NOTIFY_EMAIL_FROM" TO="$NOTIFY_EMAIL_TO" SUBJECT="$SUBJECT" BODY="$BODY" python3 -c '
import json, os
print(json.dumps({
    "from": os.environ["FROM"],
    "to": [os.environ["TO"]],
    "subject": os.environ["SUBJECT"],
    "text": os.environ["BODY"],
}))
')"

curl -fsS https://api.resend.com/emails \
  -H "Authorization: Bearer $RESEND_API_KEY" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD"
