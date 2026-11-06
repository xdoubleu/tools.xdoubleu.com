#!/usr/bin/env bash
# Exercises scripts/opencode_agent.sh with a stub curl (via
# factory_agent_headers.sh): `export` prints an export line for a fresh token,
# `--static` bakes the token into a throwaway repo's opencode.json and sets
# skip-worktree there, and both need FACTORY_AGENT_CLIENT_ID.
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELPER="$ROOT_DIR/scripts/opencode_agent.sh"

fail=0
pass() { printf 'PASS: %s\n' "$1"; }
failn() { printf 'FAIL: %s\n' "$1"; fail=1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/repo"

# Stub curl: answers discovery + token endpoint, records a failure toggle.
cat > "$WORK/bin/curl" <<'STUB'
#!/usr/bin/env bash
case "$*" in
  *oauth-authorization-server*) echo '{"token_endpoint":"https://idp.example/oauth2/token"}' ;;
  *oauth2/token*)
    [ "${STUB_FAIL:-}" = 1 ] && exit 22
    echo '{"access_token":"tok-123"}' ;;
esac
STUB
chmod +x "$WORK/bin/curl"

# A throwaway repo in which --static rewrites opencode.json and skips it.
cp "$ROOT_DIR/opencode.json" "$WORK/repo/opencode.json"
git -C "$WORK/repo" init -q
git -C "$WORK/repo" config user.email test@example.com
git -C "$WORK/repo" config user.name test
git -C "$WORK/repo" add opencode.json

export FACTORY_AGENT_CLIENT_ID="agent-abc" FACTORY_AGENT_SECRET="s3cret"
export PATH="$WORK/bin:$PATH"

out=$("$HELPER" export)
printf '%s' "$out" | grep -q '^export FACTORY_MCP_TOKEN=tok-123$' &&
  pass "export prints the export line with the minted token" ||
  failn "export output: $out"

out=$(OPENCODE_AGENT_CONFIG="$WORK/repo/opencode.json" "$HELPER" --static)
grep -q '"Authorization": "Bearer tok-123"' "$WORK/repo/opencode.json" &&
  pass "--static bakes the token into the configured opencode.json" ||
  failn "--static left opencode.json: $(cat "$WORK/repo/opencode.json")"
grep -q '{env:FACTORY_MCP_TOKEN}' "$ROOT_DIR/opencode.json" &&
  pass "the committed opencode.json keeps the env placeholder" ||
  failn "committed opencode.json changed"
git -C "$WORK/repo" ls-files -v | grep -q '^S opencode.json' &&
  pass "--static marks opencode.json skip-worktree" ||
  failn "skip-worktree flag not set"

out=$(env -u FACTORY_AGENT_CLIENT_ID "$HELPER" export 2>&1)
[ $? -ne 0 ] && printf '%s' "$out" | grep -q FACTORY_AGENT_CLIENT_ID &&
  pass "needs a client id" || failn "ran without a client id: $out"

out=$("$HELPER" bogus 2>&1)
[ $? -ne 0 ] && printf '%s' "$out" | grep -q 'usage' &&
  pass "rejects an unknown mode" || failn "no usage on unknown mode: $out"

exit "$fail"