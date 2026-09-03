# Spec: web data flow (RSC + SWR)

- Source of truth: `web/lib/client.ts`, `web/lib/server/client.ts`, `web/lib/server/fetchers.ts`, `web/components/SWRFallback.tsx`, `web/components/SWRProvider.tsx`, `web/lib/swrKeys.ts`
- Issues: #1318

## Shape

Two parallel ConnectRPC client stacks, one per rendering context.

### `lib/client.ts` — browser transport

One shared `createConnectTransport` (`useBinaryFormat: true`, avoiding base64
bloat for ebook uploads), with `fetch` forced to `credentials: 'include'`.
`createServiceClient(service)` memoizes one client per service descriptor for the
page's lifetime — call it freely in render.

### `lib/server/client.ts` — RSC-only transport

Built per request. Manually sets the `cookie` header and forces
`cache: 'no-store'`; wrapped in React's `cache()` so every parallel fetch within
one render pass shares one transport.

**It never forwards the refresh-token cookie.** An RSC can't persist rotated
cookies, so a server-triggered refresh would invalidate the session the browser
still holds. An expired access token just 401s server-side and recovers through
the client's own SWR fetch.

It sets a **10s timeout** (Node `fetch` has none by default) so a hung API call
can't block a `force-dynamic` render forever. The browser transport stays
uncapped on purpose — slow uploads and PDF conversions.

### `lib/server/fetchers.ts` — best-effort prefetch

`fetchOrNull(fn)` makes server-side prefetching best-effort: any `ConnectError`
returns `null` and the page still renders, with client SWR taking over.

- `Unauthenticated` / `PermissionDenied` / `ResourceExhausted` are **expected and
  silent**. The last is the api's own rate limiter, which every SSR render can
  trip under bot/scanner traffic — including Next's default `/_not-found`, since
  there's no custom `not-found.tsx` opting out of the root layout's current-user
  prefetch (#1318).
- A code-`Unknown` `ConnectError` wrapping a raw fetch `TypeError` is also silent
  — a dropped connection with no server response to signal anything.
- Anything else is swallowed too, but first reported to Sentry.

## Behavior

Hydration path: a server component calls `fetchOrNull` + `createServerClient`,
then hands the result to a client boundary as SWR fallback data via
`<SWRFallback fallback={{ [swrKeys.x]: data }}>`. Non-string/tuple keys go in its
`keyed` prop and **must mirror the client hook's key exactly**.

`components/SWRProvider.tsx` does the same for the current user specifically: the
root layout server-fetches it once per request, and every `swrKeys.currentUser`
consumer (Navbar, HomeClient, settings) gets it with no loading flash, while the
hook still revalidates client-side.

`SWRFallback` deliberately **merges** with any parent fallback rather than
replacing it, so nested instances compose.

## Invariants

- **`lib/swrKeys.ts` is *the* registry of SWR cache keys.** Query hooks and
  `mutate()` invalidations must both go through it; a key literal written inline
  anywhere else silently splits the cache from its invalidator.
- The RSC transport must never forward the refresh-token cookie.
- `getApiUrl()` (`lib/env.ts`) reads `window.__ENV__.API_URL` in the browser —
  injected by an inline script in `app/layout.tsx`, since the same standalone
  build is deployed with different env per environment and Next's build-time
  `NEXT_PUBLIC_` inlining doesn't fit — and `process.env.API_URL` on the server.

## Known gaps

No custom `not-found.tsx`, so `/_not-found` still runs the root layout's
current-user prefetch and can trip the rate limiter under scanner traffic
(#1318). Handled by silencing `ResourceExhausted` rather than by opting out.
