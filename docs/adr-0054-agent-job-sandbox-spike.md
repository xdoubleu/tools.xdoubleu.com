# ADR-0054: Keep the Apple `container` job VM; prototype OpenShell, drop Substrate

- Status: Proposed
- Issues: T-652, T-721, T-736; related T-478, T-539, T-541, T-491, T-575, T-648, T-735
- Affects: `factory-runner/internal/runner/` (`Sandbox`), `factory-runner/Containerfile`
- Revisits: [adr-0043](adr-0043-factory-runner-sandbox.md)

## Context

The factory is heading for a local tool, with the server only syncing state. The job VM has recurring pain: missing tools and caches, disk filling, network limits. NVIDIA OpenShell and Google's Agent Substrate (with gVisor) are CNCF projects that could replace the sandbox layer.

The spike ran without network; the gates below were measured later on the runner Mac (T-736), together with Docker Sandboxes (`sbx`), which overlaps the same layer.

## Decision

- **Substrate: drop for the local tool.** It needs Kubernetes and pays off only with a shared compute plane. Revisit with the platform respike.
- **OpenShell: prototype, not adopt.** Adopt only if all hold:
  1. Its license is permissive and compatible, and CNCF status is confirmed.
  2. Its `vm` driver boots the job image on Apple Silicon without KVM.
  3. Host mounts work for the Go and npm caches and a persistent `/nix`.
  4. Start latency is within 1.5x of the current VM.
  5. No direct network, with credentials injected only on approved endpoints, passes `scripts/factory_sandbox_probe.sh`.
- **Keep the current VM** until then. A second backend is another `Sandbox` implementation; one with its own proxy, as OpenShell has, also replaces the egress proxy.
- **Nix flake first (T-478).** The toolchain and a size-capped shared cache fix most of the tool-drift and disk tasks on any runtime; a new runtime alone removes few of them.

## Consequences

- No code change now; the sandbox stays as ADR-0043 describes.
- No second backend is written: no candidate passes, and the gate runs needed only the CLIs.

## Prototype results

Measured 2026-11-21 (T-736) on the runner Mac: Apple M3, 8 cores, 24 GB, macOS 27.0, Apple `container` 1.6.1, the `factory-job` image (2.5 GB compressed, 4.8 GB rootfs). Cold start = create, run `true`, destroy, image already local.

| Gate | OpenShell 0.1.9 (`vm` driver) | Docker Sandboxes `sbx` 0.54.0 |
| --- | --- | --- |
| 1. License, CNCF | **Pass.** Apache-2.0 ([LICENSE](https://github.com/ai-openshell/openshell/blob/main/LICENSE)); CNCF Sandbox since 2026-09-26 ([cncf.io](https://www.cncf.io/projects/openshell/)) | **Fail.** Proprietary, "All rights reserved" ([LICENSE](https://github.com/docker/sbx-releases/blob/HEAD/LICENSE)), and every local command refuses to run until `sbx login` to a Docker account |
| 2. Boots the job image on Apple Silicon | **Pass.** libkrun microVM via Hypervisor.framework, aarch64, Node, Go and Claude Code present | Not measured (owner ruled it out on gate 1) |
| 3. Go and npm cache mounts | **Pass**, with setup: virtiofs bind mounts (read-only by default, read-write on request) need `enable_bind_mounts`, a `sandbox_uid`/`sandbox_gid` matching the host user, and a Landlock policy listing each mount. `/nix` is N/A (T-478 shipped `tools/setup-ci-tools.sh`) | Not measured |
| 4. Cold start ≤ 1.5x (median of 5) | **Fail.** From the rootfs tar: 15.65 s (15.06–18.76), 13x. From a cached registry image (512 MB default): 2.02 s, 1.7x. Apple `container`: 1.17 s (1.04–1.52) | Not measured |
| 5. No direct network, credentials only via the proxy | **Pass.** The probe's checks, ported to `openshell sandbox exec`, all pass: no direct HTTPS, HTTP or raw TCP out; the VM holds only an `openshell:resolve:env:…` placeholder; the proxy injects the secret on the approved host only, refuses it elsewhere and refuses unapproved binaries (per-binary rules); no `/Users`, read-only root, non-root writable clone | Not measured |

Setup friction worth knowing: the Homebrew install does not pick the `vm` driver by default (`compute_driver = "vm"` must be set), needs `e2fsprogs`, has no insecure-registry option for a local `http://` registry, and a rootfs tar drops the image's `ENV` (PATH, GOMODCACHE), so a job would set them itself. On the tar path, about 11.5 s of each start passes before the cached root disk is used, apparently re-reading the 4.8 GB tar. The job image could not be timed from a registry: that needs a TLS registry, and the 2.02 s small-image figure is a lower bound for it.

**Recommendation: don't adopt either; keep Apple `container`.**

- **Docker Sandboxes: drop.** Proprietary and account-gated, with Docker-managed governance profiles in the CLI. That fails gate 1 and the swappable-layers goal on its own.
- **OpenShell: don't adopt now, keep watching.** It passes four of five gates, and its egress model (per-binary rules, placeholder credentials, Landlock) is stronger than ours. It fails on start time. A backend would not be a drop-in `vm.SessionSandbox`: OpenShell brings its own proxy and credential providers, so `SandboxSpec.Proxy` and wardcase's keychain secrets would move into OpenShell. Revisit when OpenShell boots a local image without restaging it and a small image starts under 1.76 s (1.5x of 1.17 s).
