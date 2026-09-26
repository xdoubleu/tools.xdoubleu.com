#!/usr/bin/env bash
# Fails fast if the apps MCP server can't be reached or exposes none of the
# routine's tools, so a mis-wired run fails here with a clear message instead
# of the agent spinning with no usable MCP tools. Needs APPS_BASE_URL and
# TOOLS_APPS_MCP_TOKEN. Exits 0 when tools/list returns every required tool,
# 1 otherwise.
set -euo pipefail

: "${APPS_BASE_URL:?}" "${TOOLS_APPS_MCP_TOKEN:?}"

# The tools the routine's step 1 pulls must expose; if any is missing the
# agent can't do its work, so fail here rather than let it loop tool-less.
read -r -d '' required <<'EOF' || true
record_action
get_grafana_alerts
get_sentry_issues
EOF

reply=$(curl -fsS -X POST "$APPS_BASE_URL/apps/mcp" \
  -H "Authorization: Bearer $TOOLS_APPS_MCP_TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --data '{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}')

# Streamable HTTP may answer as SSE; the JSON-RPC message is the data line.
if [[ $reply == *"data: "* ]]; then
  reply=$(sed -n 's/^data: //p' <<<"$reply" | tail -n 1)
fi

missing=$(comm -23 \
  <(sort <<<"$required") \
  <(jq -r '.error // empty, (.result.tools // [] | .[].name)' <<<"$reply" 2>/dev/null | sort) \
  | paste -sd, -)

if [ -n "$missing" ]; then
  echo "preflight failed: apps MCP server missing tool(s): $missing" >&2
  echo "reply: $(jq -c . <<<"$reply" 2>/dev/null || echo "$reply")" >&2
  exit 1
fi