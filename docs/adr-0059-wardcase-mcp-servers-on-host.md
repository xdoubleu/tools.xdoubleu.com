# ADR-0059: wardcase MCP servers are device config, signed in on the host

- Status: Accepted
- Issues: T-770, T-477
- Affects: `wardcase/internal/ward/mcp*.go`, `wardcase/vm/egress_proxy.go`, `wardcase/vm/harness.go`

## Context

wardcase hard-coded the board and PostHog as its MCP servers, with a stored PostHog key, and interactive runs had MCP off. Every harness should get the MCP servers the owner configured once, without a token entering the VM ([adr-0055](adr-0055-wardcase-harness-sandbox.md), [adr-0043](adr-0043-factory-runner-sandbox.md)).

## Decision

- Servers come only from the device config: `[[mcp]]` entries (`name`, `url`, `jobs`) in config.toml. A repo can't add one; a later slice lets it opt in to configured ones.
- `wardcase mcp login|list|logout <name>` runs the MCP authorization flow (spec 2025-06-18) on the host: Protected Resource Metadata (RFC 9728), authorization server metadata (RFC 8414), dynamic client registration (RFC 7591), authorization code with PKCE and the `resource` parameter (RFC 8707) through a loopback redirect. The keychain keeps the registration and refresh token; access tokens stay in memory.
- The egress proxy serves each signed-in server at `/mcp-<name>`, fixed to its URL, and adds the bearer token. It refreshes on expiry, drops a token the server answered 401 to so the next request refreshes, and stores a rotated refresh token. A server not signed in has no route; the session start names it.
- The harness prelude writes the servers (name and proxy URL, no credential) into Claude Code's `--mcp-config` and OpenCode's `mcp` with `"oauth": false`. Names reach it in `WARDCASE_MCP_SERVERS`.
- Interactive runs get every configured server; headless runs only those with `jobs = true`.
- PostHog is no longer built in, and factory-runner stores no key for it. factory-runner reads the same config and keychain on its Mac (`session.JobMCPServers`) and gives a job only the `jobs = true` servers, through the same routes and prelude. One not signed in is logged once for the job and named in its outcome; the job still runs. The board's `/mcp` route, named `tools-apps` in the harness so the repo's `mcp__tools-apps__*` tool names resolve, stays built in and not configurable.

## Alternatives considered

- Sign in inside the VM: the token would live there.
- A static key per server: no refresh, and not what hosted servers like PostHog offer.
- Replay the request after a 401: bodies can't be replayed through the reverse proxy; the client retries.

## Consequences

- A revoked sign-in fails that server's requests with 502 until `wardcase mcp login` runs again.
- The server must support dynamic client registration and issue a refresh token.

## Repo narrowing

An optional `.wardcase/config.toml` in the session's worktree lists `mcp = [names]` (and `plugins = ["marketplace/plugin"]`, narrowing the device's marketplace plugins). The session gets the listed names the device configures, after the `jobs` filter; no file means all. A name the device lacks is a warning. Any other key is refused: a repo file can't define a server, URL or credential, so repo content (which an agent's PR can change) never adds an egress route. <!-- historical -->

## Revisit when

A server needs a pre-registered client.
