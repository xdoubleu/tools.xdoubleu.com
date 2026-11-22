# wardcase/ — any agent harness in the job VM

Runs Claude Code, OpenCode, Codex or Gemini CLI in an Apple `container` VM each run in its own git worktree of the caller's checkout → [`adr-0055`](../docs/adr-0055-wardcase-harness-sandbox.md). **Separate Go module**, imported by `factory-runner`. Experimental.

```text
cmd/wardcase/     CLI: run, --print, acp, secret set, config, mcp login|list|logout
acp/              small ACP client and agent over stdio (acptest/: a scripted agent for tests)
session/          public API: one headless session, or an interactive Chat, from a repo URL and branch
internal/ward/    device config, keychain, run loop, worktrees, clone cache, overflow handoff
vm/               the VM (sandbox*.go), egress proxy, harness adapters
```

`vm` is shared with factory-runner: a change there changes every factory job. Tests run the VM setup scripts' shell on macOS too, so keep those snippets POSIX/BSD-portable (`du -sk`, no GNU-only flags). Keep the VM's reach to the proxy's fixed routes ([`adr-0043`](../docs/adr-0043-factory-runner-sandbox.md)).

**Temp paths in tests:** the sandbox is linux, CI also runs macOS, where `/tmp` is a symlink to `/private/tmp` and git reports resolved paths. A test comparing a path git (or `os.Getwd`) returned against `t.TempDir()` must wrap the dir in `filepath.EvalSymlinks` first.

**Image:** one generic base image per release (`Containerfile`: harnesses, git, Nix, no skills, nothing project-specific), `ghcr.io/xdoubleu/wardcase:<version>`, pulled on first use. The version is stamped into the binary (`make build VERSION=x.y.z`) and neither a repo nor the config can pick another image. A checkout's `flake.nix` dev shell supplies its tools, through a shared `/nix` store under `~/.local/state/wardcase/nix` (3 GiB cap) and the proxy's read-only `/nixcache/` and `/nixsrc/` routes. Factory jobs keep the `factory-job` image.

- Each run gets a worktree under the state dir on `wardcase/<id>`, cut from HEAD; `--in-place` skips it, `--resume` reuses the latest. On exit wardcase reports what is left and removes a worktree with nothing left (`internal/ward/worktree.go`).
- `session.Runner.Run` needs no checkout: it keeps one cached clone per URL under the state dir (`internal/ward/clone.go`), fetching by URL so a rewritten `origin` can't redirect the token, runs git with hooks and fsmonitor off, cuts a worktree on the new branch from `origin/<base>`, runs overflow (`off`/`auto` + fallback; the handoff file is specified in [`HANDOFF.md`](HANDOFF.md)) in it and returns the leftover counts. The token reaches git's environment only, never the VM. `Request.Dir` instead runs in a caller's existing checkout on its current branch (`Branch`, if set, must match): no clone, no new branch, never removed, same overflow and hooks-off git. `ResumeID` continues a harness session (`Harness.Resume`; state is kept in `StateDir`), `Request.Extra` lets a caller that owns the job add VM env, a kept outcome dir, the harness's session dir, the board and MCP servers. `Result` reports the last harness's `SessionID` and, for Claude Code (`vm.ModelReporter`), `Model`.
- **Chat** (`session.Runner.Start`, `internal/ward/chat.go`): the harness's ACP server (`vm.Harness.ACP()`; OpenCode, Codex through `codex-acp`, and Gemini; not Claude Code) runs over the VM's stdio; wardcase allows every permission request. A caller may give the chat the board (`Board`: server and token source) and further MCP servers (`MCP`); without a `Board` the chat has none. `wardcase acp` serves the same session to an ADE → [`adr-0060`](../docs/adr-0060-wardcase-interactive-sessions-over-acp.md).
- **MCP servers** are `[[mcp]]` config entries, signed in with OAuth on the host (`internal/ward/mcp*.go`); the proxy serves each at `/mcp-<name>` and adds the token, so the VM never holds one → [`adr-0055`](../docs/adr-0055-wardcase-harness-sandbox.md). Names reach the harness prelude through `WARDCASE_MCP_SERVERS`; the VM is named `wardcase-<WARDCASE_RUN>`, and the runner alone sets `FACTORY_JOB`. A repo's optional `.wardcase/config.toml` (`mcp = [...]`, `plugins = [...]`) can only narrow them to names the device has, read from the session's worktree (`internal/ward/repoconfig.go`); other keys are refused → [`adr-0059`](../docs/adr-0059-wardcase-mcp-servers-on-host.md).
- `make test | lint | build`
- Real VM tests: `WARDCASE_IMAGE=factory-job:<tag> go test -tags container -run TestRealVM ./vm`
- Setup: `wardcase secret set anthropic` (a `claude setup-token` value), `openrouter`, `openai` and `gemini`, each from stdin.
