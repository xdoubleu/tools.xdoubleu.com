# ADR-0037: Replace the unmaintained `ory/fosite` with Authelia's fork, `authelia.com/provider/oauth2`

- Status: Accepted — the owner picked (c), 2026-10-26
- Issues: #2612 (audit #2596)
- Affects: `api/internal/platform/identity/oauth2as/`, `api/internal/platform/identity/oauth2as/wiring.go`, `api/go.mod`, `api/cmd/api/migrations/00058_oauth2_session_numeric_claim_times.sql`, `.github/workflows/api-vuln.yml`, `renovate.json5`; replaces the library chosen in [ADR-0006](adr-0006-embedded-oauth21-authorization-server.md)

## Context

fosite is the protocol engine of the embedded AS ([ADR-0006](adr-0006-embedded-oauth21-authorization-server.md), [ADR-0021](adr-0021-oauth-as-general-purpose-oidc-idp.md), [ADR-0025](adr-0025-machine-client-credentials-service-role.md)); registration, discovery, JWKS and consent are our own code. As of 2026-10-25:

- The last tag is v0.49.0 (2024-12) and the last master commit is 2025-07; OpenSSF Scorecard "Maintained" is 0.
- The code moved into Hydra's `fosite/`, and Ory files advisories against Hydra's modules, so a future fix would likely not flag `github.com/ory/fosite`.
- 231 of the 955 packages in `cmd/api` exist only for fosite (deprecated jaeger exporter, logrus, pop, grpc, archived `golang/mock`).

## Options

(a) and (c) were prototyped on a scratch copy and passed the `oauth2as` suite and the OAuth/MCP tests. (c) drops `go.sum` from 1046 to 294 lines and `cmd/api` from 955 to 784 packages, and links none of the deprecated dependencies; (a) keeps them.

- **(a) Hydra's in-tree fosite** (`github.com/ory/hydra/v2/fosite`, not `internal/`). Storage and strategies become provider accessors (`AuthorizeCodeStorage()`, `CommonStrategyProvider`), and `fosite.Transactional` is required (a pass-through matches today). It is a dead end, though:
  - Hydra's calver tags are invalid for the `/v2` module path, so it pins only by pseudo-version (`v2.3.1-0.20260320110106-0b84568fffcc`), and Renovate can't follow releases.
  - v26.2.0 builds outside Hydra only because it still requires a published `ory/x` (v0.0.724). Master requires `ory/x v0.0.0-00010101000000-000000000000`, which only its own `replace => ./oryx` resolves ([go.mod](https://proxy.golang.org/github.com/ory/hydra/v2/@v/v2.3.1-0.20260729093009-4174065ffb05.mod)), so `go get …@master` fails. `ory/x` stopped tagging at v0.0.729 (2025-07-07).
  - The deprecated dependencies stay.
- **(b) Fork or vendor fosite at `a5f0b09`.** It is API-compatible: master mainly adds the RFC 8628 device flow. Replacing `ory/x` (`errorsx` in 42 files, `otelx` in 9) drops the jaeger/logrus/pop/grpc chain. But we would own ~11.8k lines of security-critical code and hand-port Hydra's fixes into its diverged API.
- **(c) A maintained library:**
  - **[Authelia's fork](https://github.com/authelia/oauth2-provider)**: 65 tags since 2024-03, v0.3.3 released 2026-09-19, Scorecard Maintained 10, required by Authelia v4.39.28. No `ory/x`, otel or grpc in its [go.mod](https://proxy.golang.org/authelia.com/provider/oauth2/@v/v0.3.3.mod). It keeps v0.49's storage and strategy shape and adds RFC 7591/7592, DPoP and mTLS. The port renames `OAuth2Provider` to `Provider`, signs through a `jwt.Strategy` whose JWKS issuer uses `OIDCKeyID`, switches to `NumericDate` claims and `BCryptClientSecret`, and stubs `ClientRegistrationManager`. It is pre-1.0 and shaped by Authelia's needs.
  - **[zitadel/oidc](https://github.com/zitadel/oidc) `op`** (v3.51.16, 2026-10-15, Scorecard Maintained 10) has PKCE, client_credentials, refresh, discovery/JWKS and device flow. Its [feature table](https://github.com/zitadel/oidc/blob/v3.51.16/README.md) has no dynamic registration. Storage is a different model (`AuthRequest` IDs, with the app owning refresh rotation and reuse detection). Opaque access tokens are encrypted `tokenID:subject` strings ([token.go](https://github.com/zitadel/oidc/blob/v3.51.16/pkg/op/token.go)). Moving to it means rewriting `oauth2as` and the consent flow, and every client re-authenticates. It also requires the archived `golang/mock`.
  - **[luikyv/go-oidc](https://github.com/luikyv/go-oidc)** (v0.25.0, 2026-07-23): OpenID-certified, Dynamic OP included, with three direct dependencies. It has a single maintainer, is v0.x, and would be a full rewrite.
- **(d) Stay on v0.49.0.** It works today, but the risk is silent: fixes and advisories land on Hydra, which neither Dependabot nor govulncheck connects to our module. The deprecated dependencies stay.

## Decision

(c), Authelia's fork: it is maintained and Renovate-trackable, drops every flagged dependency, keeps issued token strings valid, and passed both suites. Migration as shipped:

1. **Fixtures** written by v0.49.0 are captured before the swap.
2. **Swap**: the root package is aliased as `fosite`, with `NewHMACCoreStrategy(cfg, "ory_%s_")` so issued tokens stay valid.
3. **Backfill** (migration 00058): v0.49 stored claim times as RFC 3339 strings and absent claims as `null`, which the fork rejects. Unmigrated rows still introspect, but refresh and code exchange fail. The Down block restores the old shape, so a rollback runs it before the old binary starts.
4. **Compatibility test** on the backfilled fixture covers introspection, refresh rotation, PKCE exchange and reuse revocation.
5. **Monitoring**: govulncheck runs daily rather than per PR.

Fallback: (a) is a one-time bridge if the fork stalls (34 lines, no data migration).

## Consequences

- ADR-0006's decisions stand; only the engine changes.
- About 18% fewer packages and a 3.6 MB smaller binary; the deprecated dependencies leave `go.mod`. `go.uber.org/mock` is required only by the fork's tests and isn't linked.
- The fork's API is pre-1.0, so Renovate doesn't auto-merge its bumps.
- A rollback without the Down block costs MCP clients and Grafana one re-login.

## Revisit when

The fork stops releasing, Ory publishes an importable fosite again, or a requirement such as UserInfo or certification favours zitadel/oidc.
