# ADR-0024: PostHog Cloud (EU) for product analytics + session replay, no consent gate

- Status: Accepted
- Issues: #1638 (part of epic #1637)
- Affects: `web/instrumentation-client.ts`, `web/components/SWRProvider.tsx`, `web/lib/env.ts`, `web/app/layout.tsx`, `web/app/settings/page.tsx`, `proto/auth/v1/auth.proto`, `api/cmd/api/connect_auth_handlers.go`, `config/deploy.web.yml`, `.kamal/secrets`, `.github/workflows/main.yml`

## Context

The Web Vitals beacon (ADR-0022) is aggregate-only; it can't show where users get
stuck. Epic #1637 wants a scheduled routine to mine per-user events and replays
for friction. Every family member is an individual account (ADR-0008).

## Decision

- **PostHog Cloud, EU region**, not self-hosted.
- **Product analytics and full session replay on by default** for every family
  member, with no consent gate — a deliberate choice. A one-line passive
  notice sits in `/settings`, with no opt-out.
- **PostHog's default masking only**, including for `health/`.
- **Client-side only**: `instrumentation-client.ts` initializes it (like
  Sentry); nothing in Go. It no-ops when `POSTHOG_KEY` is unset.
- **Identify by `user_id`**: `GetCurrentUserResponse` gained `user_id`;
  `SWRProvider.tsx` calls `posthog.identify(currentUser.userId)` when it differs
  from the current `distinct_id`.
- **Runtime env via `window.__ENV__`** (like `getSentryDsn()`), not build-time
  `NEXT_PUBLIC_*`. `POSTHOG_KEY` is listed under `env.secret:` (like
  `SENTRY_DSN_WEB`); `POSTHOG_HOST` is `env.clear:`
  (`https://eu.i.posthog.com`).

## Alternatives considered

- **Self-hosted** — another service to run for a personal-scale app.
- **`NEXT_PUBLIC_*` vars** — inconsistent with the runtime-env pattern.
- **Consent gate / opt-out / privacy policy** — explicitly out of scope (#1637).
- **Anonymous only** — loses per-person, cross-device correlation.

## Consequences

- `GetCurrentUserResponse.user_id` is public contract; use it as the stable
  user id.
- PostHog is a third-party browser dependency; a missing key means no analytics,
  not an error.
- Replay records everything default masking doesn't redact, including `health/`.

## Revisit when

A non-family user is onboarded (privacy policy/opt-out), or `health/` needs more
masking.
