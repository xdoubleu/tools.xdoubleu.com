# ADR-0039: Agents act on the factory board under their own accounts

- Status: Accepted
- Issues: #2776, #2701
- Affects: `api/internal/agentaccounts/`, `api/internal/oauth2as/seed.go`
  (`MachineSubjects`), `api/internal/mcptools/agent_tools.go`,
  `api/cmd/api/migrations/00059_*`, `scripts/factory_agent_headers.sh`

## Context

The factory board lets agents file and move tasks, but never approve work: only
a human in a browser session moves a task into Ready
([#2701](https://github.com/xdoubleu/tools.xdoubleu.com/issues/2701) decision 5).
An MCP bearer token resolves to the same `models.User` as the person's cookie
session, so an agent signed in as the person could do anything the person can.
[ADR-0025](adr-0025-machine-client-credentials-service-role.md) gave routines a
machine identity, but only one, seeded by hand.

## Decision

- A new **`agent`** role. An agent account is one `agent` user plus one
  confidential OAuth client that has only the client_credentials grant, created
  together in a transaction. The user has no usable password hash, so it can't
  sign in, and it is left out of the user list. `SetRole` and `SetAppAccess`
  refuse it, so an admin can't turn it into a person.
- The token endpoint maps a client_credentials client to its user through
  `MachineSubjects`: the static `routines` client first, then active agent
  accounts (`global.agent_accounts`). Tokens last an hour; there is no refresh
  token.
- An agent sees and calls only `factory_*` tools. `mcptools.AgentToolsOnly`
  filters the tool listing, refuses other tool calls as unknown, and refuses
  resources and prompts, so the rule holds without each tool remembering it.
- Only an admin in a browser session creates, lists or revokes accounts, and
  creating one needs a TOTP or recovery code (`VerifyCurrentFactor`). The client
  secret is shown once and stored only as a bcrypt hash.
- Revoking stamps `revoked_at` and deletes the client. Its tokens cascade away,
  so they stop working at once. The account row stays for history.
- An agent connects through a `headersHelper`
  (`scripts/factory_agent_headers.sh`): it reads the secret from the macOS
  Keychain item `factory-agent-<client id>`, mints a token and prints the
  `Authorization` header. Claude Code re-runs it on a 401, so no token is
  stored.

## Setup

```bash
security add-generic-password -s factory-agent-<client id> -a agent -w
claude mcp add-json tools-apps-agent '{"type":"http",
  "url":"https://tools.xdoubleu.com/api/apps/mcp",
  "headersHelper":"FACTORY_AGENT_CLIENT_ID=<client id> /path/to/scripts/factory_agent_headers.sh"}'
```

OpenCode takes a static header, so mint a token first
(`FACTORY_MCP_TOKEN=$(FACTORY_AGENT_CLIENT_ID=… scripts/factory_agent_headers.sh | jq -r '.Authorization | sub("Bearer "; "")')`)
and set `headers: {Authorization: "Bearer {env:FACTORY_MCP_TOKEN}"}` on the
`tools-apps` server in `opencode.json`.

## Alternatives considered

- Let an agent use the person's session with a flag: the flag can't be trusted
  once the token is the same.
- Gate each tool with a role check: one forgotten tool leaks. The middleware
  makes the rule a default.
- A static token per agent: never expires and can't be revoked without a deploy.

## Consequences

- An agent account is a shared secret: whoever holds it acts as that agent until
  it is revoked.
- An agent with no `factory_*` tools yet sees an empty tool list; the tools
  arrive with the board MCP slice (#2714).
- The step-up needs a verified TOTP factor, so an admin without one can't create
  accounts.

## Revisit when

An agent needs per-task scoping narrower than "every factory tool", or tokens
should be bound to a device.
