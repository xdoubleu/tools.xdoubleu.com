#!/usr/bin/env bash
# Opens or closes a routine's global.automated_actions row through the apps
# MCP server's record_action tool, so the workflow rather than the agent owns
# the run record. Needs APPS_BASE_URL and TOOLS_APPS_MCP_TOKEN.
#   routine_record.sh open ROUTINE TRIGGER_SOURCE           prints the row id
#   routine_record.sh close ID OUTCOME PR_URL ERROR METRICS_JSON
set -euo pipefail

: "${APPS_BASE_URL:?}" "${TOOLS_APPS_MCP_TOKEN:?}"

# call ARGUMENTS_JSON prints record_action's structured result. The server is
# stateless, so a bare tools/call needs no initialize handshake.
call() {
  local body reply
  body=$(jq -nc --argjson args "$1" '{jsonrpc: "2.0", id: 1,
    method: "tools/call", params: {name: "record_action", arguments: $args}}')
  reply=$(curl -fsS -X POST "$APPS_BASE_URL/apps/mcp" \
    -H "Authorization: Bearer $TOOLS_APPS_MCP_TOKEN" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    --data "$body")
  # Streamable HTTP may answer as SSE; the JSON-RPC message is the data line.
  if [[ $reply == *"data: "* ]]; then
    reply=$(sed -n 's/^data: //p' <<<"$reply" | tail -n 1)
  fi
  if ! jq -e '.error == null and (.result.isError | not)' <<<"$reply" >/dev/null; then
    echo "record_action failed: $(jq -c '.error // .result.content' <<<"$reply")" >&2
    return 1
  fi
  jq -c '.result.structuredContent // {}' <<<"$reply"
}

case ${1:-} in
  open)
    call "$(jq -nc --arg r "${2:?routine}" --arg t "${3:?trigger source}" \
      '{mode: "open", routine_name: $r, trigger_source: $t}')" | jq -r .id
    ;;
  close)
    call "$(jq -nc --argjson id "${2:?id}" --arg o "${3:?outcome}" \
      --arg p "${4:-}" --arg e "${5:-}" --argjson m "${6:-null}" \
      '{mode: "close", id: $id, outcome: $o, pr_url: $p, error: $e}
        + (if $m == null then {} else {metrics: $m} end)')" >/dev/null
    ;;
  *)
    echo "usage: routine_record.sh open|close ..." >&2
    exit 2
    ;;
esac
