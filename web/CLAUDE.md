# web/ — Frontend

Next.js 16 App Router app, React 19, TypeScript strict, built as a standalone
Node server (`output: 'standalone'` in `next.config.ts`, run via `node
server.js`). Run all `npm` commands from this directory.

## Layout

- `app/` — one route folder per app/domain, matching the API's service boundaries (`games/`, `books/`, `feeds/`, `recipes/`, `mealplans/`, `shoppinglist/`, `watchparty/`, plus `auth/`, `contacts/`, `monitoring/`, `dashboard/`, `settings/`, `sharing/`, `user-management/`, `oauth/consent/`). `dashboard/{games,reading}/` holds both the private (owner) and public (token-shared) Games/Reading dashboards (issue #737) — `games/`/`books/` no longer have a dashboard-shaped route of their own, only library/detail/settings pages.
- `components/` — shared cross-app components at the root (`Navbar.tsx`, `HomeClient.tsx`, `SWRFallback.tsx`, `SWRProvider.tsx`, …) plus one subfolder per domain mirroring `app/`, and `components/ui/` for shadcn-style primitives.
- `lib/` — `client.ts` (browser ConnectRPC transport), `server/` (RSC-only transport + fetchers), `swrKeys.ts` (SWR cache key registry), `env.ts`, `cn.ts`, `gen/` (generated ConnectRPC clients — read the `.proto` source instead), plus one subfolder per domain (`books/`, `recipes/`, `games/`, `watchparty/`, `oauth2as/`).
- `hooks/` — one SWR data-fetching hook file per domain.

## Data Flow (RSC + SWR)

Two parallel ConnectRPC client stacks, one per rendering context:

- **`lib/client.ts`** — browser transport. One shared `createConnectTransport` (`useBinaryFormat: true`, avoids base64 bloat for ebook uploads), `fetch` forced to `credentials: 'include'`. `createServiceClient(service)` memoizes one client per service descriptor for the page's lifetime — call it freely in render.
- **`lib/server/client.ts`** — RSC-only transport built per request. Manually sets the `cookie` header and forces `cache: 'no-store'`; wrapped in React's `cache()` so every parallel fetch within one render pass shares one transport. **Never forwards the refresh-token cookie** — an RSC can't persist rotated cookies, so a server-triggered refresh would invalidate the session the browser still holds; an expired access token just 401s server-side and recovers through the client's own SWR fetch. Sets a 10s timeout (Node `fetch` has none by default) so a hung API call can't block a `force-dynamic` render forever — the browser transport stays uncapped on purpose (slow uploads/PDF conversions).
- **`lib/server/fetchers.ts`** — `fetchOrNull(fn)` makes server-side prefetching best-effort: any `ConnectError` returns `null` (the page still renders, client SWR takes over) — `Unauthenticated`/`PermissionDenied` are expected and silent, as is a code-`Unknown` `ConnectError` wrapping a raw fetch `TypeError` (a dropped connection with no server response to signal anything), anything else is also swallowed but first reported to Sentry.

Hydration path: a server component calls `fetchOrNull` + `createServerClient`, then hands the result to a client boundary as SWR fallback data via `<SWRFallback fallback={{ [swrKeys.x]: data }}>` (`components/SWRFallback.tsx`) — non-string/tuple keys go in its `keyed` prop and must mirror the client hook's key exactly. `components/SWRProvider.tsx` does the same for the current user specifically: the root layout server-fetches it once per request and every `swrKeys.currentUser` consumer (Navbar, HomeClient, settings) gets it with no loading flash, while the hook still revalidates client-side. `SWRFallback` deliberately merges with any parent fallback rather than replacing it, so nested instances compose.

`lib/swrKeys.ts` is **the** registry of SWR cache keys — query hooks and `mutate()` invalidations must both go through it; a key literal written inline anywhere else silently splits the cache from its invalidator.

`lib/env.ts`'s `getApiUrl()` reads `window.__ENV__.API_URL` in the browser (injected by an inline script in `app/layout.tsx`, since the same standalone build is deployed with different env per environment — Next's build-time `NEXT_PUBLIC_` inlining doesn't fit) and `process.env.API_URL` on the server.

## Commands

```bash
npm run dev
npm run build                              # required before finishing web tasks, see root CLAUDE.md
npm run lint                                # eslint → tsc --noEmit → prettier --check → knip → syncpack lint
npm run lint:fix                            # eslint --fix + prettier --write
npm test                                    # jest
npm run test:cov                            # jest --coverage
npm run test:cov:diff                        # jest --coverage, then scope the report to lines changed vs origin/main
npx jest path/to/file.test.ts -t "name"     # single test
npm run generate                            # buf generate — regenerate lib/gen/ from proto (pair with `make proto/generate` in api/)
npm run generate:check                      # regenerate + fail if that changed anything uncommitted (what CI's proto-staleness check does)
```

## UI Standards

- **Mobile-first and responsive**: Tailwind breakpoints (`sm:`, `md:`, `lg:`) and relative units. No fixed-pixel widths.
- **Server Components by default**: Client Components only where interactivity (`useState`, `useEffect`, event handlers) is required. A Server Component must never import from a file that pulls in client-only hooks — even for an unrelated shared constant — Next's build-time server/client boundary check rejects it, and that check is enforced **only** by `next build`, not `tsc`/ESLint/Jest. Put constants shared across the boundary in a plain `lib/` module with no React imports.
- **shadcn/ui primitives**: every interactive control uses a `components/ui/` primitive (`Button`, `Input`, `Select`, `Textarea`, `MenuItem`, `Badge`, `Card`, `Dialog`, `Checkbox`, `RadioGroup`, `Combobox`, `Popover`, `DateInput`, `Table`). Don't hand-style raw `<button>`/`<input>`/`<select>`/`<textarea>`.
- **Class overrides**: merge with `cn()` from `lib/cn.ts` (`clsx` + `tailwind-merge`) so a `className` prop reliably overrides a primitive's defaults.
- **Clickable cards**: use `interactiveCardClass` exported from `components/ui/card.tsx` for the shared hover/ring treatment — the accent ring shows at rest (not just on hover) so cards read as interactive on touch devices with no hover state. Don't hand-roll per-card `hover:shadow`/`hover:bg` variants.
- **Tailwind v4, CSS-first theme**: no `tailwind.config.ts` anywhere — `app/globals.css` defines RGB-triple CSS variables per light/dark, exposed as utilities via `@theme inline`, plus a separate `@theme` block for shadow/radius tokens. Colors track the active scheme at runtime.
- **Light/dark**: the dark tokens key off `:root[data-theme='dark']`, not `prefers-color-scheme`. `lib/theme.ts` owns the per-device preference (`auto`/`light`/`dark` in localStorage) and exports `themeInitScript`, inlined in `app/layout.tsx`'s `<head>` so the resolved scheme is on `<html>` before first paint (no flash) and `auto` stays live via a `matchMedia` listener. The Appearance section on `/settings` is the toggle. `lib/theme.ts` deliberately has no React imports (server layout + client component both import it).
- **Async states**: page-level loading is `<p className="text-muted">Loading…</p>`; page-level errors are `<p className="text-danger">Failed to load X.</p>`. Buttons swap their label to a `…`-suffixed present-participle string while a mutation is pending (`Saving…`, `Updating…`). Always the typographic ellipsis `…`, never `...`.

## kobo-gateway Client (`lib/books/gatewayClient.ts`)

Client for the local kobo-gateway macOS menu-bar helper (`https://127.0.0.1:41132`, self-signed cert trusted on first launch; server implementation in `kobo-gateway/internal/kobogateway`, its own Go module — see `kobo-gateway/CLAUDE.md`). The browser makes all authenticated API calls itself and only hands the gateway a resulting sync URL; the gateway patches the USB-mounted Kobo's config file directly. `REQUIRED_GATEWAY_VERSION` is a floor for genuine protocol breaks — bump it alongside `GatewayVersion` in the Go code only then. `gatewayNeedsUpdate` also compares the gateway's `release` build stamp against `getKoboGatewayRelease()` (`lib/env.ts`) — the release of the kobo-gateway artifact actually bundled in the current deploy, not this web build's own `getRelease()`, since kobo-gateway's build can be cache-skipped (unchanged source, see root `CLAUDE.md`'s CI section) and legitimately lag behind — that comparison is what actually delivers *routine* releases (bug fixes that don't touch the protocol) to installed gateways.

## Static Downloads

`web/public/` does not exist in the repo — it's assembled at Docker build time. The kobo-gateway `.dmg` and raw binary are built on macOS by `build-kobo-gateway.yml` and staged into `web/public/downloads/` by `build-web.yml` itself before `docker build` runs (moved there from the now-deleted `docker.yml` merged-image assembly job in #1038); `web/Dockerfile` just `COPY`s them from there. Next standalone only serves `public/` assembled that way, so the download route 404s under `npm run dev` unless you build `kobo-gateway/` locally first and copy the artifacts in yourself.

## OAuth Consent Screen (`app/oauth/consent/`)

Server-rendered OAuth 2.1 consent screen for the apps MCP server (`/apps/mcp` on the api), driving the api's own embedded fosite authorization server directly (issue #1039 — no external Auth provider). The api's `AuthorizeHandler` (`api/internal/oauth2as/handlers.go`) redirects here with the pending authorization request's own query params (`client_id`, `scope`, `state`, `redirect_uri`, `code_challenge`, ...) echoed verbatim; the page reads the `accessToken` cookie server-side, calls `GET /oauth2/consent-info` (`lib/oauth2as/consentClient.ts`) for the client name/scope to display, and the `approveAuthorization`/`denyAuthorization` server actions (`app/oauth/consent/actions.ts`) POST the same query params plus `consent=allow|deny` back to `/oauth2/authorize`, forwarding the session cookie, then `redirect()` the browser to whatever `Location` fosite responds with (the OAuth client's own redirect_uri, carrying a code or an `access_denied` error). No env config needed beyond the existing `API_URL`.

## File Size & Splits

TypeScript/TSX files over ~300 lines need a split before adding more code:

- Components — split by UI concern (e.g. `MealPlanCalendar.tsx` → `MealPlanMealChip.tsx`, `MealPlanEntryForm.tsx`)
- Hooks — split by data domain
- Utility files — split by concern

## Testing

Jest + React Testing Library. Target ≥80% coverage on `components/`, `lib/`, `hooks/` (`lib/gen/` excluded). Run `npm run test:cov` for the full report, or `npm run test:cov:diff` to scope it to just the lines changed vs `origin/main` (line + branch %, matching what CI's `codecov/patch` check gates on) — run this before pushing. It exits non-zero on any changed file whose *changed lines* fall under 80% line/branch coverage, or that has no coverage data at all (e.g. a new file nothing imports yet, exactly the gap that used to only surface in CI). A file whose diff touches no instrumented lines is reported but never flagged. Scoping to changed lines is what keeps it honest in both directions: a one-line edit to a file with pre-existing gaps no longer fails, and a wholly untested new function added to an otherwise well-covered file no longer hides behind that file's healthy overall percentage (issue #1301). It's `tools/diff_coverage_ts.py` at the repo root, a plain-Python lcov parser sitting alongside `tools/merge_coverage.py` and `tools/diff_coverage_go.py` (the equivalents for `api`'s Go coverage profiles); it only covers files matched by `collectCoverageFrom` in `jest.config.js`.

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
