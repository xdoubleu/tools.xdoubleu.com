#!/usr/bin/env bash
# Exercises scripts/routine_preflight.sh: it must pass when the apps MCP server
# exposes the required tools for both tokens and fail when either token lacks its
# tools.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFLIGHT="$ROOT_DIR/scripts/routine_preflight.sh"

fail=0
pass() { printf 'PASS: %s\n' "$1"; }
failn() { printf 'FAIL: %s\n' "$1"; fail=1; }

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
PORT=$(( 9000 + RANDOM % 1000 ))

# A minimal streamable-HTTP MCP server. tools is a space-separated list of tool
# names to advertise on tools/list.
cat > "$WORK/mock.py" <<'PY'
import json, sys, os
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
        if body.get('method') == 'tools/list':
            # An agent token sees the board tools, the service token the rest.
            agent = self.headers.get('Authorization') == 'Bearer agent-token'
            names = os.environ.get("AGENT_TOOLS" if agent else "TOOLS", "").split()
            tools = [{"name": n, "description": "x",
                      "inputSchema": {"type": "object"}} for n in names]
            self._s({"jsonrpc": "2.0", "id": body.get("id"),
                     "result": {"tools": tools}})
        else:
            self._s({"jsonrpc": "2.0", "id": body.get("id"), "result": {}})
HTTPServer(('127.0.0.1', int(os.environ["PORT"])), H).serve_forever()
PY

export APPS_BASE_URL="http://127.0.0.1:$PORT" TOOLS_APPS_MCP_TOKEN="probe-token"
export FACTORY_AGENT_MCP_TOKEN="agent-token"
export PORT
BOARD="factory_list_repos factory_list_tasks factory_get_task factory_create_task factory_comment_task"
run_preflight() {
  local tools="$1" agent_tools="${2-$BOARD}"
  TOOLS="$tools" AGENT_TOOLS="$agent_tools" python3 "$WORK/mock.py" > "$WORK/srv.log" 2>&1 &
  local sp=$!
  sleep 0.5
  if "$PREFLIGHT" > "$WORK/out.log" 2>&1; then local rc=0; else local rc=$?; fi
  kill "$sp" 2>/dev/null; wait "$sp" 2>/dev/null
  return "$rc"
}

bash -n "$PREFLIGHT" || { failn "routine_preflight.sh bash -n"; exit 1; }
pass "routine_preflight.sh parses"

if run_preflight "record_action get_grafana_alerts get_sentry_issues"; then
  pass "succeeds when required tools are exposed"
else
  failn "failed although required tools were exposed"
fi

if run_preflight ""; then
  failn "succeeded although no tools were exposed"
else
  pass "fails when no tools are exposed"
fi

if run_preflight "record_action get_grafana_alerts get_sentry_issues" ""; then
  failn "succeeded although the agent account sees no board tools"
else
  pass "fails when the agent token lacks the board tools"
fi

if run_preflight "record_action get_grafana_alerts get_sentry_issues" "factory_list_tasks"; then
  failn "succeeded although the agent account lacks factory_create_task"
else
  pass "fails when one board tool is missing"
fi

if env -u FACTORY_AGENT_MCP_TOKEN "$PREFLIGHT" > "$WORK/out.log" 2>&1; then
  failn "succeeded without a board token"
else
  pass "fails without FACTORY_AGENT_MCP_TOKEN"
fi

exit "$fail"