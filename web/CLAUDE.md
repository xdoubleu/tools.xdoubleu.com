# web/ — Frontend

Next.js 16 App Router application built as a standalone Node server (`output: 'standalone'`, run via `node server.js` in Docker). Run all `npm` commands from this directory.

## Stack

| Concern        | Library                                                   |
| -------------- | --------------------------------------------------------- |
| Framework      | Next.js 16, React 19, TypeScript strict                   |
| Styling        | Tailwind CSS v4 (CSS-first theme via `@theme` in `app/globals.css`; no `tailwind.config.ts`) + shadcn/ui |
| API client     | ConnectRPC (`@connectrpc/connect-web`)                    |
| Data fetching  | SWR                                                       |
| Error tracking | Sentry (`@sentry/nextjs`)                                 |
| Testing        | Jest + React Testing Library                              |
| Linting        | ESLint (eslint-config-next), Prettier, tsc --noEmit, knip |

## Key Paths

- `app/` — App Router pages and layouts
- `components/` — Reusable React components (shadcn/ui primitives in `components/ui/`)
- `lib/` — Utilities and ConnectRPC client setup
- `lib/swrKeys.ts` — **The** registry of SWR cache keys. Query hooks and `mutate()` invalidations must both use it; never write a key literal inline (drifted keys silently split the cache).
- `lib/server/` — Server-side ConnectRPC client for React Server Components (`createServerClient` forwards the request's cookies — except the refresh token — and `fetchOrNull` makes prefetching best-effort)
- `lib/gen/` — Generated TypeScript ConnectRPC clients from buf (committed; only regenerate after editing `.proto` files). **Do not read `lib/gen/`** to discover RPC types or method signatures — read the `.proto` source in `proto/` instead.
- `hooks/` — SWR data-fetching hooks
- `lib/books/gatewayClient.ts` — Client for the local kobo-gateway (macOS helper at `http://127.0.0.1:41132`; Go source in `api/internal/kobogateway`). `KoboSetup` probes it and prefers the gateway flow (`KoboGatewaySetup`) over the File System Access flow. `REQUIRED_GATEWAY_VERSION` here must track `GatewayVersion` in the Go code — bumping the Go side without this constant means clients never trigger the self-update.

## Static Downloads

`web/public/` does not exist in the repo. The kobo-gateway binary is cross-compiled in `web/Dockerfile` (gateway-builder stage) and copied to `public/downloads/` next to `server.js` — Next standalone only serves `public/` assembled that way, so the download 404s under `npm run dev`. If `web/public/` ever gains committed files, `web/Dockerfile` needs an extra `COPY web/public ./public`.

## Data Flow (RSC + SWR)

Every route's initial data is fetched in an **async server component** and injected into the SWR cache; the client components keep using their SWR hooks unchanged:

1. The page calls `createServerClient(Service)` (`lib/server/client.ts`) and wraps fetches in `fetchOrNull` (`lib/server/fetchers.ts`). Any `ConnectError` yields `null` — the page still renders and the client-side SWR fetch takes over.
2. Results are passed to `<SWRFallback fallback={{ [swrKeys.x]: data }}>` (`components/SWRFallback.tsx`), which merges into the parent SWRConfig fallback. Non-string keys (tuples/objects) go in its `keyed` prop and must mirror the client hook's initial key **exactly**.
3. The root layout server-fetches the current user once per request and provides it via `components/SWRProvider.tsx` for every `swrKeys.currentUser` consumer (Navbar, HomeClient, settings).
4. SWR still revalidates on mount — mutations, live polling, and websockets behave exactly as before.

**Never forward the refresh token server-side** (already enforced in `lib/server/client.ts`): RSCs cannot persist rotated cookies, so a server-triggered refresh would invalidate the browser's session. Expired access tokens 401 on the server and recover through the browser's SWR fetch.

`getApiUrl()` (`lib/env.ts`) resolves `window.__ENV__.API_URL` in the browser and `process.env.API_URL` on the server — the server URL is used by `lib/server/client.ts`.

Client-side, `createServiceClient` (`lib/client.ts`) memoizes one client per service descriptor; call it freely in render.

## Common Commands

```bash
npm run build                             # Production build
npm run lint                              # ESLint + Prettier + tsc + knip
npm test                                  # Run all tests
npm run test:cov                          # With coverage report
npm run test:single MealPlanCalendar      # By filename
npm run test:single -- -t "renders correctly"  # By test name
npm run generate                          # Regenerate lib/gen/ from proto definitions
                                          # (pair with `make proto/generate` in api/)
```

## UI Standards

- **Mobile-first and responsive**: use Tailwind responsive breakpoints (`sm:`, `md:`, `lg:`) and relative units. No fixed-pixel widths.
- **Server Components by default**: use Client Components only where interactivity (`useState`, `useEffect`, event handlers) is required.
- **Minimal friction**: prefer SWR / React state updates over full page reloads. Use optimistic UI where appropriate; avoid unnecessary loading states.
- **shadcn/ui primitives**: every interactive control must use a `components/ui/` primitive — `Button`, `Input`, `Select`, `Textarea`, `MenuItem` (dropdown rows), `Badge`, `Card`, `Dialog`. Do **not** hand-style raw `<button>`/`<input>`/`<select>`/`<textarea>`. The only sanctioned raw elements are genuinely different patterns: ARIA `role="tab"` strips, sidebar nav lists, and native checkbox/color/file inputs.
- **Consistent shape**: interactive controls are `rounded-xl` (buttons, inputs, selects, textareas), small/tiny controls `rounded-lg`, containers (cards, dialogs, dropdown panels) `rounded-2xl`, status badges `rounded-full`. Never use bare `rounded` — always pick a value from the scale.
- **Clickable cards**: navigable cards (Links or `onClick` divs that look like cards) share one hover treatment via `interactiveCardClass` from `components/ui/card.tsx` — `cn(interactiveCardClass, 'block p-4')`. Do not hand-roll per-card `hover:shadow`/`hover:bg` variants.
- **Class overrides**: merge classes with `cn()` from `lib/cn.ts` (clsx + tailwind-merge) so a `className` prop reliably overrides a primitive's defaults (e.g. `<Input className="w-16" />` beats the default `w-full`). All `components/ui/` primitives already use `cn`.
- **Links that look like buttons**: use `<Button asChild><Link …/></Button>` rather than re-styling the `<Link>`.
- **Page shell**: every page wraps its content in `PageContainer` (never a raw `<main>` — the root layout already renders `<main>`). Standard padding is `p-6`; width via `size="narrow"` or a `max-w-*` override in `className`.
- **Page titles**: `<h1 className="text-3xl font-bold">` (add `mb-6` when the title stands alone, `leading-tight` for long content titles). No per-app title styles.
- **Async states**: page-level loading is `<p className="text-muted">Loading…</p>`; page-level errors are `<p className="text-danger">Failed to load X.</p>`. Inside cards/lists use the `py-16 text-center text-sm text-muted` pattern. Always the typographic ellipsis `…`, never `...`.

## File Size & Splits

TypeScript/TSX files over ~300 lines need a split before adding more code:

- Components — split by UI concern (e.g. `MealPlanCalendar.tsx` → `MealPlanMealChip.tsx`, `MealPlanEntryForm.tsx`)
- Hooks — split by data domain
- Utility files — split by concern

## Testing

Jest + React Testing Library. Run `npm run test:cov` for coverage. Target ≥80% on `components/`, `lib/`, `hooks/` (excludes `lib/gen/`).
