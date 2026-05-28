# web/ — Frontend

Next.js 16 App Router application served as a static export (`output: 'export'`). Run all `yarn` commands from this directory.

## Stack

| Concern        | Library                                                   |
| -------------- | --------------------------------------------------------- |
| Framework      | Next.js 16, React 19, TypeScript strict                   |
| Styling        | Tailwind CSS + shadcn/ui                                  |
| API client     | ConnectRPC (`@connectrpc/connect-web`)                    |
| Data fetching  | SWR                                                       |
| Error tracking | Sentry (`@sentry/nextjs`)                                 |
| Testing        | Jest + React Testing Library                              |
| Linting        | ESLint (eslint-config-next), Prettier, tsc --noEmit, knip |

## Key Paths

- `app/` — App Router pages and layouts
- `components/` — Reusable React components (shadcn/ui primitives in `components/ui/`)
- `lib/` — Utilities and ConnectRPC client setup
- `lib/gen/` — Generated TypeScript ConnectRPC clients from buf (committed; only regenerate after editing `.proto` files). **Do not read `lib/gen/`** to discover RPC types or method signatures — read the `.proto` source in `proto/` instead.
- `hooks/` — SWR data-fetching hooks

## Common Commands

```bash
yarn build                                # Production build
yarn lint                                 # ESLint + Prettier + tsc + knip
yarn test                                 # Run all tests
yarn test:cov                             # With coverage report
yarn test:single MealPlanCalendar         # By filename
yarn test:single -t "renders correctly"   # By test name
yarn generate                             # Regenerate lib/gen/ from proto definitions
                                          # (pair with `make proto/generate` in api/)
```

## UI Standards

- **Mobile-first and responsive**: use Tailwind responsive breakpoints (`sm:`, `md:`, `lg:`) and relative units. No fixed-pixel widths.
- **Server Components by default**: use Client Components only where interactivity (`useState`, `useEffect`, event handlers) is required.
- **Minimal friction**: prefer SWR / React state updates over full page reloads. Use optimistic UI where appropriate; avoid unnecessary loading states.
- **shadcn/ui primitives**: reach for existing components in `components/ui/` before writing custom markup.

## File Size & Splits

TypeScript/TSX files over ~300 lines need a split before adding more code:

- Components — split by UI concern (e.g. `MealPlanCalendar.tsx` → `MealPlanMealChip.tsx`, `MealPlanEntryForm.tsx`)
- Hooks — split by data domain
- Utility files — split by concern

## Testing

Jest + React Testing Library. Run `yarn test:cov` for coverage. Target ≥80% on `components/`, `lib/`, `hooks/` (excludes `lib/gen/`).
