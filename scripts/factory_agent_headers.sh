#!/usr/bin/env bash
# headersHelper for an agent account (docs/adr-0039): mints a client_credentials
# token and prints the Authorization header as JSON, so no token is stored.
# Needs FACTORY_AGENT_CLIENT_ID; the secret is FACTORY_AGENT_SECRET or the macOS
# Keychain item "factory-agent-<client id>". FACTORY_BASE_URL overrides the
# default https://tools.xdoubleu.com/api.
set -euo pipefail

client_id="${FACTORY_AGENT_CLIENT_ID:?set FACTORY_AGENT_CLIENT_ID}"
base_url="${FACTORY_BASE_URL:-https://tools.xdoubleu.com/api}"

if [ -n "${FACTORY_AGENT_SECRET:-}" ]; then
  secret="$FACTORY_AGENT_SECRET"
else
  secret=$(security find-generic-password -s "factory-agent-$client_id" -w)
fi

token_url=$(curl -fsS "$base_url/.well-known/oauth-authorization-server" |
  jq -er .token_endpoint)
# The credentials go to curl on stdin, so the secret never shows in `ps`.
escaped_id=$(printf '%s' "$client_id" | sed 's/[\\"]/\\&/g')
escaped_secret=$(printf '%s' "$secret" | sed 's/[\\"]/\\&/g')
token=$(printf 'user = "%s:%s"\n' "$escaped_id" "$escaped_secret" |
  curl -fsS -K - -d grant_type=client_credentials "$token_url" | jq -er .access_token)

jq -nc --arg token "$token" '{Authorization: ("Bearer " + $token)}'
