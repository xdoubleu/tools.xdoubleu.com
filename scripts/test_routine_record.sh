#!/usr/bin/env bash
# Exercises scripts/routine_record.sh: open must print the real row id and
# close must succeed, whether the MCP server returns the result only as text
# Content (the bug) or with structuredContent.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RECORD="$ROOT_DIR/scripts/routine_record.sh"

fail=0
pass() { printf 'PASS: %s\n' "$1"; }
failn() { printf 'FAIL: %s\n' "$1"; fail=1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
PORT=$(( 9000 + RANDOM % 1000 ))

# A minimal streamable-HTTP MCP server. STYLE selects whether record_action's
# open result carries structuredContent or only text Content.
cat > "$WORK/mock.py" <<'PY'
import json, os
from http.server import BaseHTTPRequestHandler, HTTPServer
class H(BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def _s(self, o):
        b = json.dumps(o).encode()
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(b)))
        self.end_headers()
        self.wfile.write(b)
    def do_POST(self):
        ln = int(self.headers.get('Content-Length', 0))
        body = json.loads(self.rfile.read(ln) or b'{}')
        points = body.get('params', {}).get('arguments', {})
        if points.get('mode') == 'open':
            text = {"type": "text", "text": '{"id": 760}'}
            if os.environ.get("STYLE") == "text":
                result = {"content": [text], "isError": False}
            else:
                result = {"content": [text], "structuredContent": {"id": 760}, "isError": False}
        else:
            result = {"content": [{"type": "text", "text": "{}"}], "isError": False}
        self._s({"jsonrpc": "2.0", "id": body.get("id"), "result": result})
HTTPServer(('127.0.0.1', int(os.environ["PORT"])), H).serve_forever()
PY

export APPS_BASE_URL="http://127.0.0.1:$PORT" TOOLS_APPS_MCP_TOKEN="probe-token"
export PORT

# open must print the integer id and close must succeed. Prints a summary line.
run_cycle() {
  local style="$1"
  STYLE="$style" python3 "$WORK/mock.py" > "$WORK/$style.log" 2>&1 &
  local sp=$!
  sleep 0.5
  local id open_rc close_rc=0
  id=$("$RECORD" open "test-routine" "manual" 2> "$WORK/$style.err")
  open_rc=$?
  if [ "$open_rc" -eq 0 ] && [ -n "$id" ]; then
    "$RECORD" close "$id" "no_action_needed" "" "" null > /dev/null 2>&1 || close_rc=$?
  else
    close_rc=99
  fi
  kill "$sp" 2>/dev/null; wait "$sp" 2>/dev/null
  printf 'id=%s open_rc=%s close_rc=%s\n' "$id" "$open_rc" "$close_rc"
}

bash -n "$RECORD" || { failn "routine_record.sh bash -n"; exit 1; }
pass "routine_record.sh parses"

# Old server: open result is text Content only. Regression test: open must not
# print null.
out=$(run_cycle text)
if [[ "$out" == *"id=760 open_rc=0 close_rc=0"* ]]; then
  pass "text-only result: open prints id and close succeeds"
else
  failn "text-only result: unexpected output: $out ($(cat "$WORK/text.err" 2>/dev/null))"
fi

# New server: open result carries structuredContent.
out=$(run_cycle structured)
if [[ "$out" == *"id=760 open_rc=0 close_rc=0"* ]]; then
  pass "structuredContent result: open prints id and close succeeds"
else
  failn "structuredContent result: unexpected output: $out"
fi

exit "$fail"