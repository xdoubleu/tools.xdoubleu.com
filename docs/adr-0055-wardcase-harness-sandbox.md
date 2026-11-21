# ADR-0055: wardcase, the job VM as a front door for any agent harness

- Status: Accepted (experimental)
- Issues: T-735; related T-345, T-477, T-241, T-273
- Affects: `wardcase/` (new module), `factory-runner/internal/runner/vm.go`, `.github/workflows/`
- Builds on: [adr-0043](adr-0043-factory-runner-sandbox.md)

## Context

ADEs (Orca, Conductor) now drive Claude Code, OpenCode and others in parallel, and Orca's Bots run them unattended. None isolates the harness: Orca's bots run as the user with full access by default. A market check (2026-11-21) found sandboxes, config sync and cross-harness evals each exist separately (Docker Sandboxes, brood-box, agent-vm, rulesync, Harbor); credentials tied to an approval and evals from one's own merged work do not.

The factory already has the parts: the Apple `container` VM, the egress proxy and the harness adapters, inside `factory-runner`. The owner tried Orca and prefers the board's simpler UI for following agent work, so the board stays; the open question is whether an ADE later takes over that UI.

## Decision

- **A new module, `wardcase/`** (name checked free on npm, Homebrew, PyPI, crates.io, Docker Hub and GitHub). Package `vm` holds the VM, proxy and adapters, moved out of `factory-runner`, which keeps its names through aliases: one copy. CI builds and lints it with factory-runner's jobs.
- **`wardcase [claude|opencode] [args…]`** runs the harness on the user's terminal (`container exec --tty`) in a fresh VM; `--print` runs it headless on a prompt, the base for later evals. Any ADE uses it by pointing its agent command at it.
- **Workspace:** the repository's top level is mounted at `/work` (the start directory outside a repository), plus a worktree's shared git directory at its own path. `npm ci` is skipped, as Linux builds would land in the user's checkout. Sessions persist per checkout under ~/.local/state/wardcase, so `--resume` works.
- **Reach:** the proxy carries the model APIs and the read-only registries, nothing else: no board, PostHog or push route, and the harnesses' MCP config is off (`FACTORY_MCP=off`). Claude Code's `setup-token` sits in the VM and can reach only Anthropic through the proxy, adr-0043's conservative route; OpenRouter's key stays on the host. Secrets are wardcase's own keychain items.
- **Device config** in ~/.config/wardcase/config.toml: `order`, `overflow`, `image`, `opencode.model`. Harness versions are the image's.
- **Overflow:** a usage limit is the only trigger. `ask` prompts on a terminal, `auto` goes on, `off` stops; headless runs treat `ask` as `off`. The next harness gets a handoff file in its session directory: the git state and Claude's conversation (the user's messages and its replies, no tool output, from its last compact summary on). Interactive Claude's limit is read from its transcript after it exits.

## Consequences

- The factory runs unchanged; T-273 should reuse this overflow for runner jobs.
- Another harness needs an image layer, a proxy route and an adapter with `Interactive`, `Command` and `LimitFrom`. Codex has all three: the image pins `@openai/codex`, the proxy's `/openai` route adds the key from the `openai` keychain item (the VM holds a placeholder), and `codex` is opt-in in the config's `order`. Hermes is skipped: its CLI and whether it takes a proxied base URL couldn't be confirmed offline, so add it once someone checks.
- OpenCode's limit is OpenRouter's out-of-credits and rate-limit error events (no reset time), and an OpenCode handoff carries git state only. The Codex and OpenCode limit lines are modelled on the CLIs' documented shapes, not captured from a real limit hit.
- Revisit when an ADE's UI replaces the board for the owner, or when wardcase gets its own repo (T-345).
