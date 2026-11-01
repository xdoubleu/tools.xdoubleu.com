# ADR-0043: Jobs run in one Apple `container` VM each, on a host-only network behind a host proxy

- Status: Proposed. The choices below rest on documentation; the Mac checks
  (`scripts/factory_sandbox_probe.sh` and the manual list in it) have not been run.
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

## Decision (proposed)

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
  leak) is never reachable.
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
- **Image:** git, Go, Node, Postgres, the `claude` CLI and `opencode`, the repo's
  marketplace plugins, built with `container build` from a pinned `Containerfile`
  and rebuilt on a schedule or when it changes; the job runs as a non-root user.
- **CI:** GitHub's macOS runners can't run VMs, so the runner talks to a `Sandbox`
  interface and CI tests a fake. What stays manual: the probe script and the
  `claude -p` checks.

## What is documented, and what the probe must show

Documented: the flags above, one VM per container, Apache-2.0, macOS 26 only,
`--internal`, `claude setup-token` scope and `ANTHROPIC_BASE_URL`.

Unverified, and the probe's job: that a VM on an `--internal` network can reach the
host gateway address (the proxy) and nothing else, including no other host
listener; that a blocked host really fails; that a non-root uid can write a
bind-mounted clone and the file lands owned by you; start-up time per job; that
`claude -p` works through the base URL, with plugins and skills loaded; and
whether the heartbeat's long-poll survives sleep and wake. Apple's docs describe
macOS 15 networking as limited and call the project under active development with
possible breaking changes, so pin the CLI version.

## Alternatives considered

- **Lima** (CNCF, Apache-2.0, `vz` driver, virtiofs mounts): mature and the
  fallback if `container` networking misbehaves. It runs a VM, not a VM per
  container, and its network isolation needs `socket_vmnet` plus extra setup.
- **Tart** (Functional Source License, Apache-2.0 after two years; owned by OpenAI
  since 2026): permits internal use, strong for macOS CI images, but aimed at
  macOS guests, and its egress controls weren't documented where I looked.
- **Claude Code's `/sandbox` or `srt`:** process-level, Bash tool only, a beta.
- **An in-VM firewall** (the reference devcontainer's iptables allowlist): kept as
  defence in depth at most; the agent inside could reconfigure it.

## Consequences

- One more daemon-side component: the Go proxy, with its own tests and allowlist.
- Jobs can't install dependencies the lockfiles don't name.
- The sandbox depends on macOS 26 and a young Apple tool.

## Revisit when

The probe fails on networking (switch to Lima), Anthropic answers on credential
intermediation, or `container` changes its networking or CLI.
