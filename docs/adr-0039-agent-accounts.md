# ADR-0039: Agents act on the factory board under their own accounts

- Status: Accepted; the "only `factory_*` tools" rule is superseded by
  [ADR-0047](adr-0047-first-class-agent-users.md)
- Issues: #2776, #2701
- Affects: `api/internal/agentaccounts/`, `api/internal/oauth2as/seed.go`
  (`MachineSubjects`), `api/internal/mcptools/agent_grants.go`,
  `api/cmd/api/migrations/00059_*`, `scripts/factory_agent_headers.sh`,
  `scripts/opencode_agent.sh`, `opencode.json`

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
- An agent sees and calls only the tools and resources of the apps it was
  granted; `mcptools.AgentGrants` filters the listing and refuses the rest as
  unknown, so the rule holds without each tool remembering it
  ([ADR-0047](adr-0047-first-class-agent-users.md)).
- Only an admin in a browser session creates, lists or revokes accounts, and
  creating one needs a TOTP or recovery code (`VerifyCurrentFactor`). The client
  secret is shown once and stored only as a bcrypt hash.
- Revoking stamps `revoked_at` and deletes the client. Its tokens cascade away,
  so they stop working at once. The account row stays for history.
- **A job gets an account of its own** (`global.agent_accounts.job_id` and
  `task_id`, migration 00060). The server makes it when the runner claims the job,
  hands its client id and secret over on the claim, and revokes it when the job
  completes, is stopped or is lost; a sweep on every claim and board read revokes
  any a revoke missed. Its `factory_*` tools reach that task only: other tasks
  answer as if they don't exist, it can't move tasks (the server moves its task
  from the job's outcome), and it can still file into Backlog. Job accounts stay
  out of the admin's list, and a revoked one keeps its scope, so a request in
  flight when the job ends is never widened. A job that can't get an account is
  failed, not run without one.
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

To rotate the secret, an admin calls `RotateAgentAccountSecret` (TOTP step-up,
as for create). The old secret keeps working for 24 hours (a second rotation waits until then; revoke to end a leaked secret), so replace the
Keychain item (`security add-generic-password -U …`) inside that window
([adr-0047](adr-0047-first-class-agent-users.md)).

OpenCode takes a static `{env:…}` header, resolved from the service's process
env at launch. The committed `opencode.json` already declares a `tools-apps-agent`
server with `"oauth": false` and a `{env:FACTORY_MCP_TOKEN}` header — no token in
the file. Mint a token before launching, and export it:

```bash
eval "$(FACTORY_AGENT_CLIENT_ID=<client id> scripts/opencode_agent.sh)"
```

`oauth: false` is required: without it a remote MCP server defaults to the OAuth
flow and signs the session in as the person, silently ignoring the header, so the
session looks connected but lists the owner's user tools instead of `factory_*`.
If that happens the header was dropped (or the token expired); the server answers
identically either way. A person session lists user tools while an agent session
lists only `factory_*`, so a fresh, correctly-exported token is the check. For a
one-shot static-token session, `opencode_agent.sh --static` bakes the fresh token
into `opencode.json` and marks it skip-worktree, so the committed skeleton never
hardcodes one. `scripts/opencode_agent.sh` wraps
`scripts/factory_agent_headers.sh`; see `make opencode/agent`.

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
