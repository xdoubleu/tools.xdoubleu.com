# wardcase/ — any agent harness in the job VM

Runs Claude Code, OpenCode or Codex in an Apple `container` VM on the caller's own checkout → [`adr-0055`](../docs/adr-0055-wardcase-harness-sandbox.md). **Separate Go module**, imported by `factory-runner`. Experimental.

```text
cmd/wardcase/     CLI: run, --print, secret set, config
internal/ward/    device config, keychain, run loop, overflow handoff
vm/               the VM (sandbox*.go), egress proxy, harness adapters
```

`vm` is shared with factory-runner: a change there changes every factory job. Keep the VM's reach to the proxy's fixed routes ([`adr-0043`](../docs/adr-0043-factory-runner-sandbox.md)).

**Image:** one generic base image per release (`Containerfile`: harnesses, git, Nix, no skills, nothing project-specific), `ghcr.io/xdoubleu/wardcase:<version>`, pulled on first use. The version is stamped into the binary (`make build VERSION=x.y.z`) and neither a repo nor the config can pick another image. A checkout's `flake.nix` dev shell supplies its tools, through a shared `/nix` store under `~/.local/state/wardcase/nix` (3 GiB cap) and the proxy's read-only `/nixcache/` and `/nixsrc/` routes. Factory jobs keep the `factory-job` image.

- `make test | lint | build`
- Real VM tests: `WARDCASE_IMAGE=factory-job:<tag> go test -tags container -run TestRealVM ./vm`
- Setup: `wardcase secret set anthropic` (a `claude setup-token` value), `openrouter` and `openai`, each from stdin.
