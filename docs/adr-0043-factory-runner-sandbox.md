# ADR-0043: Jobs run in one Apple `container` VM each, on a host-only network behind a host proxy

- Status: Accepted (owner's decision, in chat): Apple's own tooling, no third-party VM layer
- Issues: #2704, #2701 (decisions 11 and 12)
- Affects: `factory-runner/` (the sandbox interface #2706 builds), future
  `Containerfile` for the job image

## Context

The runner executes an agent that must be assumed hostile
([#2701](https://github.com/xdoubleu/tools.xdoubleu.com/issues/2701) decision
11). Per job it needs a fresh Linux VM that holds only the job's clone and
session directory, reaches nothing but the model provider and the job's MCP path,
and keeps credentials out of reach. Docker Desktop and Claude Code's own
`/sandbox` are out: the first is a shared VM, and the second covers only the Bash
tool and reads anywhere by default.

## Decision

- **VM: Apple `container`** (Apache-2.0, macOS 26, Apple silicon). Each container
  is its own lightweight VM, so the boundary is a hypervisor's, per job. Its CLI
  has what the sandbox needs: `--rm`, `--read-only`, `--tmpfs`, `--mount
  type=bind,source=…,target=…,readonly`, `--user`/`--uid`/`--gid`, `--cap-drop
  ALL`, `--cpus`, `--memory`, and `container build` for the job image.
- **Egress: a host-only network and a host proxy.** `container network create
  --internal` ("restrict to host-only network") gives the VM no route out; the
  runner's own Go proxy listens on that network's gateway address and is the only
  place it can send traffic. The proxy allows the model provider and the job's MCP
  path on tools.xdoubleu.com and nothing else. No `github.com`: the host clones.
- **Registries: no network at all.** The host fills a Go module cache and an npm
  cache from the repo's lockfiles (`go mod download`, `npm ci`) and mounts them
  read-only; the VM runs with `GOPROXY=off` and `npm ci --offline`. A registry
  that fetches whatever the client names (the `proxy.golang.org/attacker.example/…`
  leak) is never reachable. The egress proxy follows the Go proxy's module-zip redirect to `storage.googleapis.com` itself, and only that host.
- **No Docker daemon.** `make lint/infra` and `make grafana/verify` need Docker, so CI
  runs them, not the job.
- **Credentials: the token reaches the VM through an allowlisted route only.**
  Claude Code sends `CLAUDE_CODE_OAUTH_TOKEN` last in its credential order and
  honours `ANTHROPIC_BASE_URL`, so the VM can talk to the host proxy over plain
  HTTP with a dummy token, and the proxy can set the real one on the way to
  `api.anthropic.com` (no TLS interception needed). Anthropic's terms bar
  developers from collecting, storing or intermediating Claude.ai credentials, and
  allow an end user to sign in to the unmodified binary with their own plan. A
  local proxy that swaps the owner's own token is a grey area, so **start with the
  conservative route**: put the real `setup-token` value in the VM's environment.
  With egress limited to the proxy, a hostile agent can send it only to
  Anthropic, and `setup-token` credentials can only make model requests. Move to
  the swap only after Anthropic confirms it. OpenRouter's key has no such
  restriction and is always swapped by the proxy.
- **Image:** wardcase's base image (agent CLIs, git, Nix; [adr-0055](adr-0055-wardcase-harness-sandbox.md)), pulled by version ([adr-0042](adr-0042-factory-runner-distribution.md)), run as a non-root user. The toolchain is the checkout's `flake.nix` dev shell (Go, Node, linters, Postgres, Playwright, Ruby; [adr-0054](adr-0054-agent-job-sandbox-spike.md)), fetched into the host's shared `/nix` store, the same one interactive wardcase uses. No image is built from this repo. The VM starts idle on a per-job `--internal` network, the proxy binds the gateway, and the job runs in it with its environment in a 0600 file, never argv.
- **Proxy:** `EgressProxy` serves fixed routes only: the model providers (the VM's own token passed through, or the real key swapped in) and `/mcp` with a job-scoped token the host mints. Any other path or non-canonical encoding is refused, and the request's host never picks the destination.
- **Registries:** the host's Go and npm caches are mounted when configured; dependencies resolve through three more fixed GET/HEAD routes (`/goproxy/`, `/npm/`, `/rubygems/` for bundler via a `~/.bundle/config` mirror written at exec, as the mirror key is no valid env var name) with no credentials, and the lockfiles and `GOSUMDB` still verify them. Without a proxy the VM stays offline.
- **CI:** GitHub's macOS runners can't run VMs, so the runner talks to a `Sandbox` interface and CI tests a fake. The probe script (`scripts/factory_sandbox_probe.sh`) and the `claude -p` checks stay manual.

## Accepted limits

- Commit authors are set by environment variables the agent can override, so an author is a label, not proof; the App opens the pull request.
- Once the Claude token is in the VM the agent can read it and send it to Anthropic, the conservative route above.
- Apple's tool is young and may break its CLI or networking, so pin the version.

## Alternatives considered

- **Lima** (CNCF, Apache-2.0, `vz` driver, virtiofs mounts): mature, but not
  Apple's own tooling, which the owner wants to stay on. It runs a VM, not a VM per
  container, and its network isolation needs `socket_vmnet` plus extra setup.
- **Tart** (Functional Source License, Apache-2.0 after two years; owned by OpenAI
  since 2026): permits internal use, strong for macOS CI images, but aimed at
  macOS guests, and its egress controls weren't documented where I looked.
- **Claude Code's `/sandbox` or `srt`:** process-level, Bash tool only, a beta.
- **An in-VM firewall** (the reference devcontainer's iptables allowlist): kept as
  defence in depth at most; the agent inside could reconfigure it.

## Consequences

- One more component: the Go proxy, with its own tests and allowlist.
- The sandbox depends on macOS 26 and a young Apple tool.

## Revisit when

The probe fails on networking (then fix it on Apple's tooling, for example with a
host firewall rule on the network's subnet, or revisit this ADR), Anthropic answers
on credential intermediation, or `container` changes its networking or CLI.

wardcase adds two read-only routes for Nix flakes, `/nixcache/` (cache.nixos.org) and `/nixsrc/` (codeload.github.com) → [adr-0055](adr-0055-wardcase-harness-sandbox.md).

## Update: sandbox fixes target the flake

A sandbox-fix branch changes only the repo's `flake.nix`/`flake.lock` (a new flake if
none); the task waits on that PR, not a new job image.
