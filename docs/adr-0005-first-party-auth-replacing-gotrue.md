# ADR-0005: Replace Supabase GoTrue with first-party auth in `api`

- Status: Accepted
- Issues: #1039
- Affects: `api/internal/auth/`, `api/cmd/api/migrations/00017`–`00019`, `infra/`

## Context

Auth was backed by Supabase GoTrue (`supabase-community/auth-go`) running as its
own container provisioned by Tofu. Every token resolution meant a network round
trip to that container, and the whole session lifecycle was owned by a component
the repo didn't control.

## Decision

Auth is **first-party as of #1039** — no external Auth provider.
`Service`/`LocalService` live in `internal/auth/service.go`, backed by `api`'s own
`auth` Postgres schema (`migrations/00017_auth_schema.sql`,
`00018_auth_oauth2.sql`).

- **Password auth**: bcrypt (`golang.org/x/crypto/bcrypt`) hashes stored in
  `auth.users`. `ForgotPassword`/`ResetPasswordWithToken` deliver a one-time reset
  link via `internal/mailer` (Resend), reusing its `ErrNotConfigured`
  degrade-gracefully semantics rather than adding a second email path.
- **Sessions**: self-issued HS256 JWT access tokens (`JWT_SECRET`, verified
  locally — no more network round trip) carrying `sub`/`aal`/`exp`, plus opaque
  refresh tokens stored SHA-256-hashed in `auth.refresh_tokens` and **rotated on
  every use** (old row deleted, new one inserted), so
  sign-out/password-change/MFA-unenroll can actually revoke a session. A
  stateless JWT alone can't be revoked without a blocklist.
- **2FA**: TOTP via `pquerna/otp` (`internal/auth/mfa.go`), secrets encrypted at
  rest via the existing `internal/crypto.Sealer`. `ChallengeMFA` is a thin stub
  returning a synthetic challenge ID purely to preserve the existing two-step
  `ChallengeMFA`→`VerifyMFA` call shape — pquerna/otp is stateless, unlike
  GoTrue's old server-side challenge object; `totp.Validate` is what actually
  verifies.
- **Recovery codes** (`auth.recovery_codes`, bcrypt-hashed, single-use) are
  net-new — GoTrue never had these — generated on first TOTP enrollment and via
  `RegenerateRecoveryCodes`.

### The cutover was automatic, not a runbook

Migration `00017_auth_schema.sql` detected a GoTrue-shaped legacy `auth` schema
(via `auth.instances`, a table name only GoTrue/Supabase ever created) and renamed
it to `auth_gotrue_legacy` before creating the new tables. A since-removed
`internal/legacyauth.Migrate` then copied existing users' bcrypt password hashes
and verified TOTP factors across, idempotently, on every `api` boot until the
schema was dropped. Once stable in production, `auth_gotrue_legacy` was dropped by
`00019_drop_auth_gotrue_legacy.sql`.

The `gotrue` container has been removed from `infra/` entirely — `api` never
talks to it. See `infra/README.md`'s "GoTrue is gone" section.

## Alternatives considered

### Staying on GoTrue

Rejected: a network round trip per token resolution, no recovery codes, and a
session lifecycle owned by a component outside the repo — while the app already
owned a Postgres schema and a mailer it could reuse.

### A manual migration runbook for the cutover

Rejected in favor of the self-detecting migration + idempotent boot-time copy, so
no operator step could be forgotten or half-applied.

## Consequences

- `JWT_SECRET` is now a load-bearing deploy secret (see
  `convention-deploy-secrets.md`).
- Revocation depends on the refresh-token table, so refresh-token rotation must
  stay write-through; skipping the delete/insert reintroduces unrevokable
  sessions.
- Token resolution is local, so a per-token TTL cache sits in front of it — see
  `api/CLAUDE.md`'s Auth notes for the invalidation rules that follow.

## Revisit when

An external identity provider (SSO, social login at scale) becomes a real
requirement rather than a hypothetical one.
