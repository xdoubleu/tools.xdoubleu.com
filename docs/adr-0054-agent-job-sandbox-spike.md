# ADR-0054: Keep the Apple `container` job VM; prototype OpenShell, drop Substrate

- Status: Proposed
- Issues: T-652; related T-478, T-539, T-541, T-491, T-575, T-648
- Affects: `factory-runner/internal/runner/` (`Sandbox`), `factory-runner/Containerfile`
- Revisits: [adr-0043](adr-0043-factory-runner-sandbox.md)

## Context

The factory is heading for a local tool, with the server only syncing state. The job VM has recurring pain: missing tools and caches, disk filling, network limits. NVIDIA OpenShell and Google's Agent Substrate (with gVisor) are CNCF projects that could replace the sandbox layer.

The spike ran without network, so OpenShell's license and CNCF status, its macOS microVM driver, its mount and cache support and its credential model are **unverified**. Only the repo side is checked.

## Decision

- **Substrate: drop for the local tool.** It needs Kubernetes and pays off only with a shared compute plane. Revisit with the platform respike.
- **OpenShell: prototype, not adopt.** Adopt only if all hold:
  1. Its license is permissive and compatible, and CNCF status is confirmed.
  2. Its `vm` driver boots the job image on Apple Silicon without KVM.
  3. Host mounts work for the Go and npm caches and a persistent `/nix`.
  4. Start latency is within 1.5x of the current VM.
  5. No direct network, with credentials injected only on approved endpoints, passes `scripts/factory_sandbox_probe.sh`.
- **Keep the current VM** until then. A second backend is another `Sandbox` implementation; the proxy model already matches.
- **Nix flake first (T-478).** The toolchain and a size-capped shared cache fix most of the tool-drift and disk tasks on any runtime; a new runtime alone removes few of them.

## Consequences

- No code change now; the sandbox stays as ADR-0043 describes.
- Adoption needs someone with network access to run the prototype.
