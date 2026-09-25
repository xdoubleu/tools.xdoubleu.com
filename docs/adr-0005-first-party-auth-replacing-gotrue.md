# ADR-0005: Replace Supabase GoTrue with first-party auth in `api`

- Status: Accepted
- Issues: #1039
- Affects: `api/internal/auth/`, `api/cmd/api/migrations/00017`–`00019`, `infra/`

## Context

Auth ran on Supabase GoTrue in its own Tofu-provisioned container: a network
round trip per token resolution, no recovery codes, and a session lifecycle the
repo didn't control.

## Decision

Auth is first-party in `internal/auth` against `api`'s own `auth` schema
(`00017_auth_schema.sql`, `00018_auth_oauth2.sql`):

- **Passwords**: bcrypt in `auth.users`; reset links via `internal/mailer`
  (Resend), degrading with `ErrNotConfigured` like other mail.
- **Sessions**: HS256 JWT access tokens (`JWT_SECRET`, verified locally) plus
  opaque refresh tokens stored SHA-256-hashed in `auth.refresh_tokens` and
  **rotated on every use**, so sign-out, password change, and MFA unenroll can
  revoke a session.
- **2FA**: TOTP via `pquerna/otp`, secrets sealed with `internal/crypto.Sealer`.
  `ChallengeMFA` returns a synthetic ID only to keep the two-step call shape;
  `totp.Validate` does the verification.
- **Recovery codes**: bcrypt-hashed, single-use (`auth.recovery_codes`).

The cutover was automatic: `00017` renamed a GoTrue-shaped schema (detected by
`auth.instances`) to `auth_gotrue_legacy`, a boot-time copy moved hashes and
TOTP factors, and `00019` later dropped the legacy schema. No `gotrue`
container remains (`infra/README.md`, "GoTrue is gone").

## Alternatives considered

- **Stay on GoTrue** — round trips, no recovery codes, external lifecycle.
- **Manual cutover runbook** — self-detecting migration left no step to forget.

## Consequences

- `JWT_SECRET` is a load-bearing deploy secret.
- Refresh-token rotation must stay write-through (delete + insert); skipping it
  makes sessions unrevokable.
- Token resolution has a TTL cache; see `api/AGENTS.md` for invalidation rules.

## Revisit when

An external identity provider (SSO, social login) becomes a real requirement.
