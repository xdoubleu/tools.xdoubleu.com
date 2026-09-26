# Convention: web UI standards

- Enforced by:
  - `npm run lint`, which runs the ESLint raw-control bans and the local `ui/*` rules in `web/eslint-rules/` (`mobile-first-classes`, `theme-tokens`, `no-classname-template`, `touch-targets`, `use-primitives`). It also runs their RuleTester tests (`test:eslint-rules`) and `generate:ui-catalog:check`.
  - `npm run build`, for the server/client boundary.
  - The `mobile-review` skill, whose `npm run mobile:audit` fails on overflow, sub-44px or crowded tap targets, and sub-16px inputs at 375x667.
  - Review, for everything else.
- Issues: #1412, #1626

## Rule

- **Mobile-first.**
  - Primitives are 44px tall below `sm` and compact above it. Fields are ≥16px.
  - No fixed pixel sizes, no `vh`/`h-screen` (use `dvh`), and no bare `grid-cols-N≥3`.
- **Server Components by default.** Use Client Components only for interactivity.
- **Primitives for every control and repeated pattern.** The inventory is [`web/components/ui/README.md`](../web/components/ui/README.md) (generated). If nothing fits, add a primitive.
  - `PageHeader` for the page `<h1>`.
  - `PageContainer size` for page width. The shell owns padding.
  - `Card`/`SectionCard`, and `Card variant="inset"`.
  - `LinkCard` for cards that navigate, with controls in its `actions` footer.
  - `Alert`, `LoadingState`/`ErrorState`/`EmptyState`, `Badge`, `SegmentedTabs`.
  - `Field`/`Label` for labelled controls.
  - `Dialog side="sheet"` for edits that should be phone-first.
  - A hidden `<input type="file">` may use `eslint-disable-next-line no-restricted-syntax` with a reason.
- **Theme tokens only.** Colours and shadows come from `app/globals.css`, which `ui/theme-tokens` reads. No `dark:`. Add a token rather than using a palette colour.
- **Class overrides** go through `cn()` (`lib/cn.ts`), never through a template literal.
- **Pending buttons** use `Saving…`. Always `…`, never `...`.
- **Pinch-zoom is disabled** for an app-like feel (the `viewport` export in `app/layout.tsx`). That makes 16px fields and 44px targets mandatory.

## Rollout

`web/eslint-rules/legacy-files.mjs` exempts files, grouped by domain, that predate the `ui/*` rules. New files are always checked.
- **Migrating a domain:** delete its section, then fix what lint reports.
- **Never add an entry.**
- **When the list is empty:** delete the file.

## Why

- `cn()` lets a `className` prop reliably override defaults; concatenation loses to Tailwind ordering.
- `LinkCard` keeps controls out of the link. A stretched `absolute inset-0` link with controls floating above it turns every near-miss into a navigation.

## Worked examples

- **Tailwind v4, CSS-first**: there is no `tailwind.config.ts`. `app/globals.css` defines RGB-triple variables per scheme via `@theme inline`, plus `@theme` for shadow and radius tokens.
- **Light/dark** keys off `:root[data-theme='dark']`.
  - `lib/theme.ts` stores `auto`/`light`/`dark` in localStorage.
  - It exports `themeInitScript`, inlined in `app/layout.tsx`'s `<head>` to avoid a flash.
  - The toggle is on `/settings`.

## What violating it looked like

- **Server/client trap:** a Server Component that imports any file pulling in client hooks fails only at `next build`, not in `tsc`, ESLint or Jest. Put shared constants in a React-free `lib/` module; that is why `lib/theme.ts` has no React imports.
- **#1412:** these rules were documented but unchecked, and the code drifted (raw `<input>` in 19 places). The ESLint selectors and the generated catalog fixed it. When adding a rule, say what fails when it's broken; if nothing does, build that check first.
- **#1626:** the regex rules only saw plain-string classNames, so drift moved into `cn()` and template literals. By then there were:
  - 145 `size="sm"` buttons under 44px
  - 13 hand-styled `<h1>` variants
  - classes naming non-existent tokens
  - a progress bar that was a tiny target inside a stretched card link

  The `ui/*` plugin tokenises every class source instead.
