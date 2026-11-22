# ADR-0061: A chat is a claimed job the runner bridges to a wardcase session

- Status: Accepted
- Issues: T-821, T-799
- Affects: `factory-runner/internal/runner/chat*.go`, `api/internal/platform/factory` (`ClaimQueued`, `EndChat`), `proto/factory/v1/runners.proto`, `wardcase/session/chat.go`
- Amends: [adr-0044](adr-0044-factory-jobs-queue.md)

## Context

A chat session is pinned to one runner and its chat job is created running, so it holds a slot ([adr-0060](adr-0060-wardcase-interactive-sessions-over-acp.md) gives the session itself). The runner needed to learn of it, run it, and report what the VM left.

## Decision

- **Claim:** `ClaimJob` first hands the runner an unclaimed chat pinned to it (`chat_sessions.claimed_at`), before the job cap applies, since its job already counts. The `ClaimedJob` carries `chat_id`, the repo, the branch and the job's own board account, and no prompt. A chat is claimed once; a restarted runner does not resume it, and the server ends it on missed check-ins.
- **Run:** the runner validates the chat like a job and needs an OpenCode Mac (Claude Code has no ACP). It opens a wardcase session whose chat request carries the board account and the device's `jobs = true` MCP servers; they are routes of the egress proxy, so the VM holds no token.
- **Bridge:** it dials `/factory/chats/{id}/runner` with the device token and `?after=<last seq>`, runs each message as a turn, and sends harness events then `idle`. It redials on failure from the last seq and drops repeated ones.
- **End:** a `stopped` frame, a stop from `CheckIn`, a refused dial or ten minutes without a socket ends it: cancel the turn, close the session, then `EndChat` with the leftover report. The server stops a chat still running and appends the report to the person's; the job is finished by the stop, so there is no `CompleteJob`.

## Alternatives considered

- A separate claim RPC for chats: a second long poll beside `ClaimJob` for no gain, since the runner already waits there.
- Resuming a chat after a restart: the VM and its turn are gone, and the transcript could not be replayed into the harness.

## Consequences

- A chat with a task gets an account scoped to that task; one without gets none of a task's reach.
- Events are redacted with the device token and the account secret before they leave the Mac.

## Revisit when

A chat must survive a runner restart.
