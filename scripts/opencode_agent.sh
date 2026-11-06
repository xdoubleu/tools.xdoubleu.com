#!/usr/bin/env bash
# Connect this OpenCode session as an agent account (docs/adr-0039). Mints a
# token via scripts/factory_agent_headers.sh and either prints the FACTORY_MCP_TOKEN
# export line to run before launching `opencode`, or writes the header into
# opencode.json and marks it skip-worktree for a one-shot static-token session.
# Needs FACTORY_AGENT_CLIENT_ID; the secret is FACTORY_AGENT_SECRET or the
# macOS Keychain item "factory-agent-<client id>".
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELPER="$ROOT_DIR/scripts/factory_agent_headers.sh"
# --static rewrites this; tests point it at a throwaway repo.
CONFIG="${OPENCODE_AGENT_CONFIG:-$ROOT_DIR/opencode.json}"

# Mint a fresh token. The helper reads FACTORY_AGENT_CLIENT_ID itself.
auth=$(FACTORY_AGENT_CLIENT_ID="${FACTORY_AGENT_CLIENT_ID:?set FACTORY_AGENT_CLIENT_ID}" "$HELPER")
token=$(printf '%s\n' "$auth" | jq -er '.Authorization | sub("Bearer "; "")')

case "${1:-export}" in
export)
  printf 'export FACTORY_MCP_TOKEN=%q\n' "$token"
  ;;
--static)
  # One-shot static-token session: bake the fresh token into opencode.json and
  # stop git tracking changes to it, so a committed skeleton never hardcodes it.
  jq --arg token "$token" \
    '.mcp["tools-apps-agent"].headers.Authorization = ("Bearer " + $token)' \
    "$CONFIG" > "$CONFIG.tmp" && mv "$CONFIG.tmp" "$CONFIG"
  git -C "$(dirname "$CONFIG")" update-index --skip-worktree "$(basename "$CONFIG")"
  printf 'wrote a one-shot token into %s; restart OpenCode (skip-worktree set).\n' "$CONFIG"
  ;;
*)
  printf 'usage: %s [export|--static]\n' "$0" >&2
  exit 2
  ;;
esac