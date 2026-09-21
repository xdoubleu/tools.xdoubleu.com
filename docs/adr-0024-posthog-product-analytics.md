# ADR-0024: PostHog Cloud (EU) for product analytics + session replay, no consent gate

- Status: Accepted
- Issues: #1638 (part of epic #1637)
- Affects: `web/instrumentation-client.ts`, `web/components/SWRProvider.tsx`,
  `web/lib/env.ts`, `web/app/layout.tsx`, `web/app/settings/page.tsx`,
  `proto/auth/v1/auth.proto`, `api/cmd/api/connect_auth_handlers.go`,
  `config/deploy.web.yml`, `.kamal/secrets`, `.github/workflows/main.yml`

## Context

The app already ships one telemetry path: `web/app/_components/web-vitals.tsx`
+ `web/app/metrics/route.ts` beacon Core Web Vitals to `POST /metrics`,
feeding the `web_vitals_seconds` Prometheus histogram
([`adr-0022`](adr-0022-prometheus-grafana-metrics.md)). That path is
aggregate-only, with no per-user identification and no event stream — it
cannot answer "where do users get stuck," only "how fast did this route
render." Epic #1637 wants product-analytics-driven UX discovery: real usage
telemetry mined by a scheduled routine for friction signals
(rage-clicks, drop-off, unused features), which needs per-user event capture
and session replay, not another aggregate metric.

Every family member is an individually authenticated user, not a shared
household login ([`adr-0008`](adr-0008-family-as-single-sharing-concept.md)),
so a product-analytics tool can identify real distinct people rather than one
undifferentiated household session. There is no privacy policy or
cookie-consent flow anywhere in this app today.

## Decision

- **Hosting: PostHog Cloud, EU region.** Not self-hosted — no infra to run or
  patch for a personal-scale app.
- **Tracking scope: aggregate product analytics *and* full session replay, on
  by default for every family member, no opt-in/consent gate.** A deliberate,
  informed choice despite there being no privacy policy elsewhere in the app.
- **Disclosure, not consent:** a one-line passive notice in `/settings`
  ("Usage analytics and session recording are enabled to help improve the
  app.") — no opt-out control, visibility only.
- **Masking:** PostHog's own defaults everywhere (password/sensitive input
  masking only). No extra masking or exclusion for any app, including
  `health/`.
- **Client-side only.** No server-side PostHog SDK usage, no capturing from
  Go (`api/`) — `web/instrumentation-client.ts` initializes the client the
  same way `Sentry.init` already does there, and autocapture instruments the
  whole app the moment the provider mounts once, near the root of the React
  tree. There is no meaningful per-app rollout to sequence.
- **Identification needs a real user id.** `auth.v1.GetCurrentUserResponse`
  previously exposed `role`/`app_access`/`has_mfa`/`display_name` only — none
  of them a stable per-user identifier suitable as a PostHog `distinct_id`
  (`display_name` is optional and user-editable). Added `user_id` (the
  existing `models.User.ID`, already resolved by `GetCurrentUser`'s handler)
  as a fifth field. `web/components/SWRProvider.tsx` — the client component
  that already receives `currentUser` to bridge it into the SWR cache — calls
  `posthog.identify(currentUser.userId)` once it differs from PostHog's
  current `distinct_id`, so anonymous pre-auth activity and post-auth
  activity resolve to the same person without re-fetching anything.
- **Env vars via the existing `window.__ENV__` runtime-injection pattern, not
  build-time `NEXT_PUBLIC_*`.** PostHog's own Next.js docs suggest
  `NEXT_PUBLIC_POSTHOG_KEY`/`NEXT_PUBLIC_POSTHOG_HOST`, baked in at build
  time. This repo already solved "one build, environment-specific runtime
  config" for exactly this class of value (`lib/env.ts`'s `getSentryDsn()`,
  same "public value, but environment-configured" shape as a PostHog project
  key) by injecting `window.__ENV__` from `process.env` in `app/layout.tsx`'s
  inline script, read per-request rather than baked in at image-build time.
  `POSTHOG_KEY`/`POSTHOG_HOST` follow that existing convention instead of
  introducing a second, inconsistent one.
- **`POSTHOG_KEY` is a deploy secret, `POSTHOG_HOST` is a plain env var.**
  The client key isn't a secret in the sense of an attacker-usable
  credential — it ships in the browser bundle regardless — but
  `SENTRY_DSN_WEB` already established the precedent of listing this class of
  externally-issued, per-environment value under `env.secret:` for
  consistency. `POSTHOG_HOST` is a fixed constant
  (`https://eu.i.posthog.com`, PostHog's EU Cloud ingestion host) with no
  per-environment variation, so it's `env.clear:` like `API_URL`/`WEB_URL`.

## Alternatives considered

- **Self-hosted PostHog.** Rejected — another service to run, patch, and back
  up for a personal-scale app; PostHog Cloud's free tier covers this
  workload.
- **`NEXT_PUBLIC_POSTHOG_KEY`/`NEXT_PUBLIC_POSTHOG_HOST` as PostHog's docs
  suggest.** Rejected in favor of the existing `window.__ENV__` pattern — see
  Decision above.
- **Consent gate / opt-out control / privacy-policy page.** Explicitly out of
  scope by epic #1637's own decision, not an oversight — see #1637's "Not
  this" section.
- **Skip `posthog.identify()`, rely on PostHog's anonymous `distinct_id`
  only.** Would still capture aggregate friction signals but defeats the
  epic's stated goal ("PostHog identifies real distinct people") and loses
  cross-session/cross-device correlation for the same person.

## Consequences

- `auth.v1.GetCurrentUserResponse.user_id` is now part of the public
  ConnectRPC contract every client of `AuthService.GetCurrentUser` sees, not
  just PostHog identification — any future caller can rely on it as the
  stable per-user id instead of `display_name`.
- PostHog Cloud is a new third-party dependency in the browser critical path;
  `instrumentation-client.ts` no-ops (skips `posthog.init`) when
  `POSTHOG_KEY` is unset (e.g. local dev without a configured key), so a
  missing key degrades to "no analytics," not a startup error.
- Session replay records everything PostHog's own default masking doesn't
  redact, for every family member, including `health/` — a conscious
  trade-off epic #1637 made explicitly, not a gap to close later.

## Revisit when

A privacy policy or opt-out control becomes a real requirement (e.g. a
non-family user is ever onboarded), or `health/`'s data sensitivity turns out
to warrant masking beyond PostHog's defaults — neither is expected for a
personal, family-only tool, but both were explicit non-goals here rather than
oversights.
