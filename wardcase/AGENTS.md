# wardcase/ — any agent harness in the job VM

Runs Claude Code, OpenCode or Codex in the factory's Apple `container` VM each run in its own git worktree of the caller's checkout → [`adr-0055`](../docs/adr-0055-wardcase-harness-sandbox.md). **Separate Go module**, imported by `factory-runner`. Experimental.

```text
cmd/wardcase/     CLI: run, --print, secret set, config
internal/ward/    device config, keychain, run loop, worktrees, overflow handoff
vm/               the VM (sandbox*.go), egress proxy, harness adapters
```

`vm` is shared with factory-runner: a change there changes every factory job. Keep the VM's reach to the proxy's fixed routes ([`adr-0043`](../docs/adr-0043-factory-runner-sandbox.md)).

- Each run gets a worktree under the state dir on `wardcase/<id>`, cut from HEAD; `--in-place` skips it, `--resume` reuses the latest. On exit wardcase reports what is left and removes a worktree with nothing left (`internal/ward/worktree.go`).
- `make test | lint | build`
- Real VM tests: `WARDCASE_IMAGE=factory-job:<tag> go test -tags container -run TestRealVM ./vm`
- Setup: `wardcase secret set anthropic` (a `claude setup-token` value), `openrouter` and `openai`, each from stdin.
