# web/ — Frontend

Next.js 16 App Router app, React 19, TypeScript strict, built as a standalone
Node server (`output: 'standalone'` in `next.config.ts`, run via `node
server.js`). Run all `npm` commands from this directory.

## Layout

- `app/` — one route folder per app/domain, matching the API's service boundaries (`games/`, `books/`, `feeds/`, `recipes/`, `mealplans/`, `shoppinglist/`, `watchparty/`, plus `auth/`, `family/`, `monitoring/`, `dashboard/`, `settings/`, `sharing/`, `user-management/`, `oauth/consent/`). `dashboard/{games,reading}/` holds both the private (owner) and public (token-shared) Games/Reading dashboards — `games/`/`books/` no longer have a dashboard-shaped route of their own, only library/detail/settings pages → [`docs/adr-0007-dashboard-app-owns-public-sharing.md`](../docs/adr-0007-dashboard-app-owns-public-sharing.md).
- `components/` — shared cross-app components at the root (`Navbar.tsx`, `HomeClient.tsx`, `SWRFallback.tsx`, `SWRProvider.tsx`, …) plus one subfolder per domain mirroring `app/`, and `components/ui/` for shadcn-style primitives.
- `lib/` — `client.ts` (browser ConnectRPC transport), `server/` (RSC-only transport + fetchers), `swrKeys.ts` (SWR cache key registry), `env.ts`, `cn.ts`, `gen/` (generated ConnectRPC clients — read the `.proto` source instead), plus one subfolder per domain (`books/`, `recipes/`, `games/`, `watchparty/`, `oauth2as/`).
- `hooks/` — one SWR data-fetching hook file per domain.

## Data Flow (RSC + SWR)

Two parallel ConnectRPC client stacks, one per rendering context — `lib/client.ts`
(browser, binary format, `credentials: 'include'`, memoized per service) and
`lib/server/client.ts` (RSC-only, built per request, wrapped in React's `cache()`,
10s timeout). Server components prefetch via `fetchOrNull`
(`lib/server/fetchers.ts`) and hand results to a client boundary as SWR fallback
data through `<SWRFallback>` → [`docs/spec-web-data-flow.md`](../docs/spec-web-data-flow.md).

Two rules that bite if broken:

- **The RSC transport must never forward the refresh-token cookie.** An RSC can't persist rotated cookies, so a server-triggered refresh would invalidate the session the browser still holds.
- **`lib/swrKeys.ts` is *the* registry of SWR cache keys** — query hooks and `mutate()` invalidations must both go through it; a key literal written inline anywhere else silently splits the cache from its invalidator. A `<SWRFallback>` key must mirror the client hook's key exactly.

`lib/env.ts`'s `getApiUrl()` reads `window.__ENV__.API_URL` in the browser (injected by an inline script in `app/layout.tsx`, since the same standalone build is deployed with different env per environment) and `process.env.API_URL` on the server.

## Commands

```bash
npm ci                                      # first command in a fresh worktree — node_modules/ is gitignored, so every other command here fails without it
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
npm run generate:ui-catalog                 # regenerate docs/spec-ui-primitives.md from components/ui/
npm run generate:ui-catalog:check           # regenerate + fail if stale (part of npm run lint)
```

## UI Standards

Mobile-first Tailwind (no fixed-pixel widths); Server Components by default;
every interactive control uses a `components/ui/` shadcn-style primitive —
**ESLint fails the build on a raw `<button>`/`<input>`/`<select>`/`<textarea>`
outside `components/ui/`**, so check the generated inventory in
[`docs/spec-ui-primitives.md`](../docs/spec-ui-primitives.md) before writing a
new component, and add a primitive rather than styling a raw element at a call
site. Regenerate that inventory with `npm run generate:ui-catalog` whenever
`components/ui/` changes (`npm run lint` fails if it's stale). Merge
class overrides with `cn()` from `lib/cn.ts`; clickable cards use
`interactiveCardClass` from `components/ui/card.tsx`. Page-level loading is
`<p className="text-muted">Loading…</p>`, errors
`<p className="text-danger">Failed to load X.</p>`, and pending buttons swap to a
`…`-suffixed present participle — always the typographic `…`, never `...`.
Tailwind v4 CSS-first theming (no `tailwind.config.ts`); dark tokens key off
`:root[data-theme='dark']`, owned by `lib/theme.ts` → [`docs/convention-ui-standards.md`](../docs/convention-ui-standards.md).

**A Server Component must never import from a file that pulls in client-only
hooks** — even for an unrelated shared constant. Next's server/client boundary
check rejects it, and that check is enforced **only by `next build`**, not
`tsc`/ESLint/Jest. Put constants shared across the boundary in a plain `lib/`
module with no React imports (as `lib/theme.ts` does).

## kobo-gateway Client (`lib/books/gatewayClient.ts`)

Client for the local kobo-gateway macOS menu-bar helper
(`https://127.0.0.1:41132`, self-signed cert trusted on first launch; server in
`kobo-gateway/`, its own Go module — see `kobo-gateway/CLAUDE.md`). The browser
makes all authenticated API calls itself and only hands the gateway a resulting
sync URL; the gateway patches the USB-mounted Kobo's config file directly.

`REQUIRED_GATEWAY_VERSION` is a floor for **genuine protocol breaks only** — bump
it alongside `GatewayVersion` in the Go code only then. `gatewayNeedsUpdate` also
compares the gateway's `release` stamp against `getKoboGatewayRelease()`, the
release of the artifact actually bundled in this deploy rather than web's own,
which is what delivers routine (non-protocol) updates → [`docs/adr-0004-runtime-release-env-vs-compile-stamp.md`](../docs/adr-0004-runtime-release-env-vs-compile-stamp.md).

## Static Downloads

`web/public/` does not exist in the repo — it's assembled at Docker build time.
`build-web.yml` stages the kobo-gateway `.dmg` and raw binary into
`web/public/downloads/` before `docker build` runs, and `web/Dockerfile` just
`COPY`s them. **So the download route 404s under `npm run dev`** unless you build
`kobo-gateway/` locally first and copy the artifacts in yourself → [`docs/adr-0002-kobo-gateway-ci-cache-split.md`](../docs/adr-0002-kobo-gateway-ci-cache-split.md).

## OAuth Consent Screen (`app/oauth/consent/`)

Server-rendered OAuth 2.1 consent screen for the apps MCP server, driving the
api's own embedded fosite authorization server directly. Needs no env config
beyond the existing `API_URL` → [`docs/spec-oauth-consent-screen.md`](../docs/spec-oauth-consent-screen.md).

## File Size & Splits

TypeScript/TSX files over ~300 lines need a split before adding more code:

- Components — split by UI concern (e.g. `MealPlanCalendar.tsx` → `MealPlanMealChip.tsx`, `MealPlanEntryForm.tsx`)
- Hooks — split by data domain
- Utility files — split by concern

## Testing

Jest + React Testing Library. Target ≥80% coverage on `components/`, `lib/`,
`hooks/` (`lib/gen/` excluded). `npm run test:cov` for the full report, or
`npm run test:cov:diff` to scope it to lines changed vs `origin/main` — **run
this before pushing**; it matches what CI's `codecov/patch` gates on and exits
non-zero on a changed file under 80% or with no coverage data at all
→ [`docs/adr-0013-diff-scoped-coverage.md`](../docs/adr-0013-diff-scoped-coverage.md).

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
