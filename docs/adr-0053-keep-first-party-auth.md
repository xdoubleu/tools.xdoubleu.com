# ADR-0053: Keep first-party sign-in and harden it

- Status: Accepted
- Issues: T-687; hardening T-681, T-682, T-683, T-684, T-685
- Affects: `api/internal/platform/auth/`, `web/app/auth/`
- Supersedes: [adr-0051](adr-0051-zitadel-for-people-sign-in.md)

## Context

ADR-0051 moved people's sign-in to self-hosted Zitadel. It never ran in production, and no one was linked to it. Every option kept the MCP authorization server, agent accounts and delegation in the api. A provider would only take over passwords, TOTP, recovery codes and reset mail, which are the simpler parts. An audit found the existing code sound: bcrypt with timing-safe unknown-email checks, lockout covering MFA, TOTP replay protection, hashed single-use recovery and reset tokens, and sessions revoked on password change.

## Decision

- People keep signing in through `internal/platform/auth` ([adr-0005](adr-0005-first-party-auth-replacing-gotrue.md) stands).
- The Zitadel container, OIDC sign-in, Zitadel user API and import are removed. Deactivating a person stays as a first-party feature.
- The gaps found are fixed in place: passkeys, lockout in Postgres, an Origin check on cookie-authenticated writes, a 72-byte maximum and breached-password check, and bcrypt cost 12.

## Alternatives considered

- **Self-hosted Zitadel**: about 512 MB, sign-in depends on another service, and its login UI has had account-takeover advisories.
- **A hosted provider (Auth0, Zitadel Cloud, WorkOS)**: agents would be apps, not principals, unless Auth0's early-access agent identities ship to free plans.

## Revisit when

A provider offers agents as first-class principals, with delegation, on a free plan.
