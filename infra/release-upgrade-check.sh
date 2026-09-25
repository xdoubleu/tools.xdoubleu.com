#!/usr/bin/env bash
# Emails via Resend when `do-release-upgrade -c` reports a new Ubuntu LTS.
# Run as root by release-upgrade-check.timer.
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

# json.dumps escapes $OUTPUT safely for Resend's JSON body.
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
