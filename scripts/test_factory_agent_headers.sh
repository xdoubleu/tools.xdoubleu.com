#!/usr/bin/env bash
# Exercises scripts/factory_agent_headers.sh with a stub curl: it must print the
# token as an Authorization header, send the client credentials, and fail
# without printing a header when the token can't be minted.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELPER="$ROOT_DIR/scripts/factory_agent_headers.sh"

fail=0
pass() { printf 'PASS: %s\n' "$1"; }
failn() { printf 'FAIL: %s\n' "$1"; fail=1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir "$WORK/bin"

# Records its arguments and answers the discovery and token requests.
cat > "$WORK/bin/curl" <<'STUB'
#!/usr/bin/env bash
echo "$*" >> "$STUB_LOG"
case "$*" in
  *oauth-authorization-server*) echo '{"token_endpoint":"https://idp.example/oauth2/token"}' ;;
  *oauth2/token*)
    [ "${STUB_FAIL:-}" = 1 ] && exit 22
    echo '{"access_token":"tok-123"}' ;;
esac
STUB
chmod +x "$WORK/bin/curl"

run() {
  PATH="$WORK/bin:$PATH" STUB_LOG="$WORK/log" FACTORY_AGENT_CLIENT_ID="agent-abc" \
    FACTORY_AGENT_SECRET="s3cret" "$HELPER" "$@"
}

out=$(run) && [ "$out" = '{"Authorization":"Bearer tok-123"}' ] &&
  pass "prints the bearer token as an Authorization header" ||
  failn "unexpected output: $out"

grep -q -- '-u agent-abc:s3cret' "$WORK/log" &&
  grep -q 'grant_type=client_credentials' "$WORK/log" &&
  grep -q 'https://idp.example/oauth2/token' "$WORK/log" &&
  pass "posts the client credentials to the discovered token endpoint" ||
  failn "token request was $(cat "$WORK/log")"

: > "$WORK/log"
PATH="$WORK/bin:$PATH" STUB_LOG="$WORK/log" FACTORY_AGENT_CLIENT_ID="agent-abc" \
  FACTORY_AGENT_SECRET="s3cret" FACTORY_BASE_URL="https://example.test/api" "$HELPER" >/dev/null
grep -q 'https://example.test/api/.well-known/oauth-authorization-server' "$WORK/log" &&
  pass "FACTORY_BASE_URL moves the discovery request" ||
  failn "discovery request was $(cat "$WORK/log")"

out=$(STUB_FAIL=1 run 2>/dev/null) && failn "a failed mint exited 0" ||
  { [ -z "$out" ] && pass "fails without printing a header when minting fails" ||
    failn "printed '$out' on failure"; }

out=$(PATH="$WORK/bin:$PATH" STUB_LOG="$WORK/log" FACTORY_AGENT_SECRET=x "$HELPER" 2>&1) &&
  failn "ran without a client id" ||
  { printf '%s' "$out" | grep -q FACTORY_AGENT_CLIENT_ID &&
    pass "needs a client id" || failn "no client id message: $out"; }

exit "$fail"
