# ADR-0043: Jobs run in one Apple `container` VM each, on a host-only network behind a host proxy

- Status: Accepted (owner's decision, in chat): Apple's own tooling, no third-party
  VM layer. The network and isolation checks of
  `scripts/factory_sandbox_probe.sh` pass on macOS 27.0 with `container` 1.6.1; the
  manual list in it is still open.
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
  `ContainerSandbox` starts the VM idle on a per-job `--internal` network, lets the
  proxy bind the gateway, then runs the job in it with `container exec` and its
  environment in a 0600 file, never argv. Claude's config sits in `/opt/claude-config`
  and is copied into a new session directory (with `cp -R`: `-a` fails on the
  bind mount's owner).
  The proxy (`EgressProxy`) serves three fixed routes on the gateway: `/anthropic/`
  passes the VM's own token through, `/openrouter/` gets the real key, and exactly
  `/mcp` gets the job account's token, which the host mints and refreshes. Any other
  path, a `.` or `..` segment, or a non-canonical encoding is refused, paths otherwise
  reach the upstream as sent, and the request's host never picks the destination. A real VM reached it and nothing else.
- **CI:** GitHub's macOS runners can't run VMs, so the runner talks to a `Sandbox`
  interface and CI tests a fake. What stays manual: the probe script and the
  `claude -p` checks.

## What is documented, and what the probe must show

Documented: the flags above, one VM per container, Apache-2.0, macOS 26 only,
`--internal`, `claude setup-token` scope and `ANTHROPIC_BASE_URL`.

Verified by the probe: a VM on an `--internal` network reaches a host listener
bound to the gateway address, and direct egress fails; a non-root uid (100) writes a
bind-mounted clone and the file lands owned by the host user; the root filesystem
and a read-only mount refuse writes; no `/Users`; a job's VM starts and ends in
under a second.

- **Pick the subnet.** `container` chose `192.168.128.0/24`, which overlapped the
  Mac's LAN; the host then routed the gateway address to the router and the VM
  reached nothing. The runner passes `--subnet` from a private range that no host
  route overlaps.
- **The gateway exists only while a VM runs.** The host bridge appears with the first
  VM on the network, so the proxy binds after that VM starts (or the runner keeps a
  placeholder VM).
- **Bind to the gateway address only.** A wildcard listener on the host may be
  reachable from the VM; that is untested.

Verified with the real image (`go test -tags container`): the seeded session
directory shows the image's marketplaces and plugins, although `CLAUDE_CONFIG_DIR`
replaces the whole config dir.

The real `claude` 2.1.285 and `opencode` 2.2.2, run in the VM against a fake
upstream, send their model calls through the proxy and reach the board's MCP
endpoint through `/mcp`: Claude with its own token passed through, OpenCode with the
host's key in place of its placeholder, and the board with a token the VM never
holds. OpenCode reads its per-job config from `OPENCODE_CONFIG`; the repo's own
`tools-apps` server stays in its config and is unreachable from the VM.

Still unverified: a model call that succeeds with the owner's real tokens (and so
`claude -p` end to end, and `claude --resume` inside the VM); whether the heartbeat's
long-poll survives sleep and wake; and wildcard listeners. OpenCode `--session`
resume, the top-level `sessionID` and sessions under `XDG_DATA_HOME` hold with
`@opencode/cli` 2.2.2, which must be the only package providing `opencode` in the
image. Apple's docs call the project under active development with possible
breaking changes, so pin the CLI version.

Two limits are accepted. Commit authors are set by environment variables the agent
can override, so an author is a label, not proof; the App opens the pull request.
And the Claude token is injected only when the job has a proxy, but once there the
agent can read it and send it to Anthropic, the conservative route above.

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

- One more daemon-side component: the Go proxy, with its own tests and allowlist.
- The image bakes in every Go module's downloads (a `file://` `GOPROXY`, with
  `GOMODCACHE` on the tmpfs), web/'s npm cache and python3. Dependencies that changed
  since the build resolve through two more fixed proxy routes, `/goproxy/` and
  `/npm/`: public registries, GET/HEAD only, no credentials. `GOSUMDB` and the
  lockfiles still verify what comes back. Without a proxy the VM stays offline.
- The sandbox depends on macOS 26 and a young Apple tool.

## Revisit when

The probe fails on networking (then fix it on Apple's tooling, for example with a
host firewall rule on the network's subnet, or revisit this ADR), Anthropic answers
on credential intermediation, or `container` changes its networking or CLI.
