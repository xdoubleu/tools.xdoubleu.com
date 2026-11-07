# Convention: web UI standards

- Enforced by:
  - `npm run lint`, which runs the ESLint raw-control bans and the local `ui/*` rules in `web/eslint-rules/` (`mobile-first-classes`, `theme-tokens`, `no-classname-template`, `touch-targets`, `use-primitives`, `field-labels`, `dialog-description`). It also runs their RuleTester tests (`test:eslint-rules`) and `generate:ui-catalog:check`.
  - `npm run build`, for the production bundle.
  - The `mobile-review` skill, whose `npm run mobile:audit` fails on overflow, sub-44px or crowded tap targets, and sub-16px inputs at 375x667.
  - Review, for everything else.
- Issues: #1412, #1626

## Rule

- **Mobile-first.**
  - Primitives are 44px tall below `sm` and compact above it. Fields are ≥16px.
  - No fixed pixel sizes, no `vh`/`h-screen` (use `dvh`), and no bare `grid-cols-N≥3`.
- **Primitives for every control and repeated pattern.** The inventory is [`web/components/ui/README.md`](../web/components/ui/README.md) (generated). If nothing fits, add a primitive.
  - `PageHeader` for the page `<h1>`. Its `actions` are all bordered: `PageHeaderLink`/`PageHeaderSettingsLink` to navigate, `secondary` buttons for commands. Only create (default) and delete (destructive) stand out.
  - `PageContainer size` for page width. The shell owns padding.
  - `Card`/`SectionCard`, and `Card variant="inset"`.
  - `LinkCard` for cards that navigate, with controls in its `actions` footer.
  - `Alert`, `LoadingState`/`ErrorState`/`EmptyState`, `Badge`, `SegmentedTabs`.
  - `Field`/`Label` for labelled controls. A field without a visible label gets `aria-label`; a placeholder is not a label.
  - `Alert tone="danger"` for errors, or `role="alert"` on an inline error beside its control, so they are announced.
  - `Dialog side="sheet"` for edits that should be phone-first.
  - `DialogDescription` goes after `DialogHeader`, or wrapped with the title in a `div`; as a direct child it lands beside the title and squeezes it (`ui/dialog-description`).
  - A hidden `<input type="file">` may use `eslint-disable-next-line no-restricted-syntax` with a reason.
- **Theme tokens only.** Colours, shadows and corner radii come from `app/globals.css`, which `ui/theme-tokens` reads. No `dark:`. Add a token rather than using a palette colour. Corners stay squared: `rounded-full` is only for square circles (`size-*`, equal `h-`/`w-`), never a pill.
- **Warm glass + editorial look.**
  - Palette is warm paper and ink with a terracotta accent (`--accent-rgb`), in light and dark. Chrome (navbar, footer, thumb and tab bars) is `bg-glass` with `backdrop-blur-xl` and `shadow-glass`.
  - Display type is the self-hosted Fraunces serif (`font-display`); body stays sans.
  - Radii are the `rounded-xl`/`2xl` tokens (continuous-curvature feel), with no accent-ring aura on focus or hover.
  - Motion is the spring spec: `ease-spring`, `duration-press` for taps (`active:scale-[0.97]`) and `duration-settle` for entrances, with plain fades under `motion-reduce`. Reuse these tokens, not ad-hoc curves.
- **Phone layout.**
  - `Dialog` docks as a bottom sheet on phones with a drag handle; pull down or fling to dismiss.
  - A page's one primary action is a `ThumbAction` (docked in the thumb zone) with a `ThumbActionSpacer` at the end of the page.
  - Apps with real sections get `BottomTabs`, wired in `components/AppTabs.tsx` (books, dashboard). It keeps the top brand bar, shows on phones only, and sets `--bottom-offset` so `ThumbAction` docks above it. Add an app by adding its sections there; a single-flow app (trains) gets none.
- **Class overrides** go through `cn()` (`lib/cn.ts`), never through a template literal.
- **Pending buttons** use `Saving…`. Always `…`, never `...`.
- **Pinch-zoom is disabled** for an app-like feel (the viewport meta in `web/index.html`). That makes 16px fields and 44px targets mandatory.

## Why

- `cn()` lets a `className` prop reliably override defaults; concatenation loses to Tailwind ordering.
- `LinkCard` keeps controls out of the link. A stretched `absolute inset-0` link with controls floating above it turns every near-miss into a navigation.

## Worked examples

- **Tailwind v4, CSS-first**: there is no JS config file. `app/globals.css` defines RGB-triple variables per scheme via `@theme inline`, plus `@theme` for shadow and radius tokens.
- **Light/dark** keys off `:root[data-theme='dark']`.
  - `lib/theme.ts` stores `auto`/`light`/`dark` in localStorage.
  - `public/theme-init.js` repeats its resolution in `index.html`'s `<head>` to avoid a flash.
  - The toggle is on `/settings`.

## What violating it looked like

- **#1412:** these rules were documented but unchecked, and the code drifted (raw `<input>` in 19 places). The ESLint selectors and the generated catalog fixed it. When adding a rule, say what fails when it's broken; if nothing does, build that check first.
- **#1626:** the regex rules only saw plain-string classNames, so drift moved into `cn()` and template literals. By then there were:
  - 145 `size="sm"` buttons under 44px
  - 13 hand-styled `<h1>` variants
  - classes naming non-existent tokens
  - a progress bar that was a tiny target inside a stretched card link

  The `ui/*` plugin tokenises every class source instead.
