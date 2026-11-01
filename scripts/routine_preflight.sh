#!/usr/bin/env bash
# Fails fast if the apps MCP server can't be reached or exposes none of the
# routine's tools, so a mis-wired run fails here with a clear message instead
# of the agent spinning with no usable MCP tools. Needs APPS_BASE_URL,
# TOOLS_APPS_MCP_TOKEN (the service client: observability tools) and
# FACTORY_AGENT_MCP_TOKEN (the routines' agent account: board tools). Exits 0
# when tools/list returns every required tool for both, 1 otherwise.
set -euo pipefail

: "${APPS_BASE_URL:?}" "${TOOLS_APPS_MCP_TOKEN:?}" "${FACTORY_AGENT_MCP_TOKEN:?}"

# check TOKEN LABEL REQUIRED...: tools/list with TOKEN must list every REQUIRED
# tool. An agent token sees only factory_* tools and a service token none of
# them, so each token is checked against its own tools.
check() {
  local token=$1 label=$2 required=$3 reply missing
  reply=$(curl -fsS -X POST "$APPS_BASE_URL/apps/mcp" \
    -H "Authorization: Bearer $token" \
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
    echo "preflight failed: $label missing tool(s): $missing" >&2
    echo "reply: $(jq -c . <<<"$reply" 2>/dev/null || echo "$reply")" >&2
    return 1
  fi
}

# The tools the routine's steps pull must be exposed; if any is missing the
# agent can't do its work, so fail here rather than let it loop tool-less.
read -r -d '' service_tools <<'EOF' || true
record_action
get_grafana_alerts
get_sentry_issues
EOF
read -r -d '' board_tools <<'EOF' || true
factory_list_repos
factory_list_tasks
factory_get_task
factory_create_task
factory_comment_task
EOF

check "$TOOLS_APPS_MCP_TOKEN" "apps MCP server (service client)" "$service_tools"
check "$FACTORY_AGENT_MCP_TOKEN" "apps MCP server (routines agent account)" "$board_tools"
