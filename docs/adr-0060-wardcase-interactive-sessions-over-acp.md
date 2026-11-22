# ADR-0060: wardcase serves interactive sessions over ACP, and allows every permission

- Status: Accepted
- Issues: T-800, T-799
- Affects: `wardcase/acp/`, `wardcase/session/chat.go`, `wardcase/internal/ward/chat.go`, `wardcase/cmd/wardcase/acp.go`, `wardcase/vm/harness.go`
- Amends: [adr-0056](adr-0056-components-not-a-self-hostable-platform.md)'s layers

## Context

wardcase ran a harness headless on one prompt ([adr-0055](adr-0055-wardcase-harness-sandbox.md)). The factory's chat (T-799) needs a session that takes several prompts, streams the harness's updates and ends on request. An ADE such as Zed can already drive agents over the Agent Client Protocol (ACP), JSON-RPC on stdio.

## Decision

- **Interactive mode** in `wardcase/session`: `Runner.Start` takes a repo URL and branch and returns a `Chat` (in its own worktree and VM, as for a headless run). `Prompt` returns the stop reason while `OnEvent` receives every `session/update`; `Cancel` stops the turn; `Close` destroys the VM and returns the leftover report.
- **The VM runs the harness's ACP server over stdio**, started through the sandbox's attach (no tty). `vm.Harness.ACP()` gives the argv or nil. OpenCode runs `opencode acp` on the same config prelude as a headless run.
- **Claude Code has none.** Anthropic's Agent SDK terms keep the subscription's Claude Code out of a product that serves others, so a chat on it is refused. Codex has no ACP server of its own.
- **Every `session/request_permission` is answered with allow**, `allow_once` before `allow_always`; a request with no allowing option is cancelled. The sandbox is the boundary ([adr-0043](adr-0043-factory-runner-sandbox.md)), and no one is there to ask in an unattended chat. Each answer is an event (`Kind: "permission"`) and a note on stderr.
- **`wardcase acp [opencode]`** is an ACP agent over the caller's stdio, one session per process, in a worktree of the directory the ADE names in `session/new`. An ADE adds wardcase as a custom agent and gets the sandbox. Permissions are answered inside wardcase and not forwarded. On hangup wardcase closes the session and reports what is left on stderr.
- **A small JSON-RPC client and agent of our own** (`wardcase/acp`: `initialize`, `session/new`, `session/prompt`, `session/update`, `session/request_permission`, `session/cancel`), not `github.com/coder/acp-go-sdk`. The sandbox has no network, so the SDK's licence and API could not be checked, and the surface needed is a few hundred lines. The client offers no file or terminal capabilities, as the agent uses its own tools in the VM. Lines on the stream that are not JSON objects are skipped, because the exec shares stdout and stderr.
- **Amends ADR-0056:** the factory gains a chat on top of its board, so it is no longer only the layer an ADE replaces; and wardcase becomes an ACP agent that an ADE can use. wardcase still never depends on the factory.

## Alternatives considered

- Parse OpenCode's headless JSON per prompt: no cancel, and a new process per turn loses its context.
- Forward permissions to the caller: nobody can answer in an unattended chat, and the VM already bounds what an allowed call can reach.
- Adopt `acp-go-sdk`: revisit once its licence and API can be checked.

## Consequences

- An allowed tool call can change anything in the worktree and reach the proxy's fixed routes, as in a headless run.
- stdout and stderr of the harness share one stream, so a harness that prints a bare JSON object to stderr would be read as a message.
- One `wardcase acp` process serves one session.

## Revisit when

Another harness ships an ACP server, or Anthropic's terms allow Claude Code in a chat.
