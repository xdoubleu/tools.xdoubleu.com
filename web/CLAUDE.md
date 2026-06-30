# web/ — Frontend

Next.js 16 App Router application served as a static export (`output: 'export'`). Run all `npm` commands from this directory.

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

## File Size & Splits

TypeScript/TSX files over ~300 lines need a split before adding more code:

- Components — split by UI concern (e.g. `MealPlanCalendar.tsx` → `MealPlanMealChip.tsx`, `MealPlanEntryForm.tsx`)
- Hooks — split by data domain
- Utility files — split by concern

## Testing

Jest + React Testing Library. Run `npm run test:cov` for coverage. Target ≥80% on `components/`, `lib/`, `hooks/` (excludes `lib/gen/`).
