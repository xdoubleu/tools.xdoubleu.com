# ADR-0037: Replace the unmaintained `ory/fosite` with Authelia's fork, `authelia.com/provider/oauth2`

- Status: Proposed — the owner picks the option in review; the migration gets its own issue
- Issues: #2612 (audit #2596)
- Affects: `api/internal/oauth2as/`, `api/cmd/api/oauth2as.go`, `api/go.mod`; revisits the library choice in [ADR-0006](adr-0006-embedded-oauth21-authorization-server.md)

## Context

fosite is the protocol engine of the embedded AS ([ADR-0006](adr-0006-embedded-oauth21-authorization-server.md), [ADR-0021](adr-0021-oauth-as-general-purpose-oidc-idp.md), [ADR-0025](adr-0025-machine-client-credentials-service-role.md)). Registration (RFC 7591), metadata/discovery, JWKS, consent and rejection logging are our own code. fosite supplies:

| Area | fosite API used |
|---|---|
| Composition (`server.go`) | `compose.Compose` with the `OAuth2AuthorizeExplicit`, `OAuth2RefreshTokenGrant`, `OAuth2ClientCredentialsGrant`, `OAuth2PKCE`, `OpenIDConnectExplicit`, `OpenIDConnectRefresh`, `OAuth2TokenIntrospection` factories; `CommonStrategy` (HMAC core, OIDC, `jwt.DefaultSigner`); `fosite.Config` |
| Endpoints (`handlers.go`, `resolver.go`) | `OAuth2Provider`: `NewAuthorizeRequest/Response`, `NewAccessRequest/Response`, `Write*`, `IntrospectToken` |
| Storage (`storage.go`) | `fosite.Storage`/`ClientManager`, `oauth2.CoreStorage`, `oauth2.TokenRevocationStorage`, `pkce.PKCERequestStorage`, `openid.OpenIDConnectRequestStorage` |
| Model | `Request`, `DefaultSession`, `openid.DefaultSession`, `jwt.IDTokenClaims`/`Headers`, `DefaultClient`, `Arguments` |
| Errors (`observe.go`) | `RFC6749Error`, `ErrorToRFC6749Error`, `ErrInactiveToken` (reuse detection), `ErrNotFound`, `ErrInvalidatedAuthorizeCode`, `openid.ErrNoSessionFound`, `ErrAccessDenied`, `ErrRequestUnauthorized`, `ErrUnauthorizedClient` |

State of fosite on 2026-10-25:

- Last tag v0.49.0 (2024-12-12), last master commit `a5f0b09` (2025-07-03) ([proxy](https://proxy.golang.org/github.com/ory/fosite/@v/master.info)); OpenSSF Scorecard "Maintained" 0 ([deps.dev](https://deps.dev/project/github/ory%2Ffosite)).
- The code moved into [Hydra's `fosite/`](https://github.com/ory/hydra/tree/v26.2.0/fosite) ("Merge branch 'fosite-monorepo'", Hydra 25.4.0, 2025-11-07, [CHANGELOG](https://github.com/ory/hydra/blob/v26.2.0/CHANGELOG.md)).
- No advisory against fosite since 0.34.1 in 2021 ([OSV](https://osv.dev/list?q=github.com%2Fory%2Ffosite&ecosystem=Go)), and govulncheck on `api` is clean. Ory now files advisories against Hydra's modules ([GO-2026-4807](https://pkg.go.dev/vuln/GO-2026-4807)), so a future fix to fosite code would likely not flag `github.com/ory/fosite`.
- 231 of the 955 packages in `cmd/api`'s build are there only for fosite, mostly via `ory/x/otelx`: the deprecated `otel/exporters/jaeger`, logrus, pop, grpc, plus the archived `golang/mock`.

## Options

(a) and (c) were prototyped on a scratch copy of `api`, then run against the `internal/oauth2as` suite (57 tests) and `cmd/api`'s OAuth/MCP tests on Postgres.

| | today: fosite v0.49.0 | (a) Hydra v26.2.0 | (c) Authelia v0.3.3 |
|---|---|---|---|
| `api/go.sum` lines | 1046 | 490 | 294 |
| packages in `cmd/api` | 955 | 943 | 784 |
| stripped binary | 54.2 MB | 54.0 MB | 50.6 MB |
| links jaeger, logrus, pop, grpc, `ory/x` | yes | yes | none |
| non-import lines changed, tests included | — | 34 | 55 |
| test suites | pass | pass, imports only changed | pass, two test lines changed |
| tokens issued before the switch | — | valid | valid with prefix `ory_%s_` |
| sessions stored before the switch | — | claim types unchanged | `openid` session JSON fails to decode |

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

Proposed: (c), Authelia's fork. It is maintained and Renovate-trackable, drops every flagged dependency, keeps issued token strings valid (stored sessions need step 4), and passed both suites. Migration:

1. **Monitoring, whatever is picked**: run `govulncheck ./...` in CI (none does today), on a schedule or non-blocking so a new disclosure doesn't turn every PR red.
2. **Fixtures**: before swapping, capture tokens and `auth.oauth2_*` rows written by v0.49.0 as `testdata/`.
3. **Swap**: alias the root package as `fosite`, apply the adaptations above, and use `NewHMACCoreStrategy(cfg, "ory_%s_")`.
4. **Backfill** `request->'session'` claim times from RFC 3339 to Unix seconds, dropping zero values; the alternative is one forced re-login for MCP clients and Grafana. Test it against the step 2 fixtures.
5. **Verify**: both suites plus a fixture-compatibility test, then after deploy one MCP connect and refresh and one Grafana SSO login, watching `oauth2 request rejected` logs.

The repo has no conformance tests. One manual run of the OpenID conformance suite against a local stack is feasible, but it isn't a CI gate: its Basic OP plan expects a UserInfo endpoint, which ADR-0021 rejected.

Fallback: (a) is a one-time bridge if the fork stalls (34 lines, no data migration).

## Consequences

- ADR-0006's decisions stand; only the engine changes.
- About 18% fewer packages and a 3.6 MB smaller binary; the deprecated dependencies leave `go.mod`.
- The fork's API is pre-1.0, so its Renovate bumps need review and shouldn't auto-merge.

## Revisit when

The fork stops releasing, Ory publishes an importable fosite again, or a requirement such as UserInfo or certification favours zitadel/oidc.
