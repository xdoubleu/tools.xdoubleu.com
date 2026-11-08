# Convention: web UI standards

- Enforced by:
  - `npm run lint`, which runs the ESLint raw-control bans and the local `ui/*` rules in `web/eslint-rules/` (`mobile-first-classes`, `theme-tokens`, `no-classname-template`, `touch-targets`, `use-primitives`, `field-labels`, `dialog-description`, `control-overrides`, and the warn-level `card-padding` and `shared-patterns` until each app is migrated). It also runs their RuleTester tests (`test:eslint-rules`) and `generate:ui-catalog:check`.
  - `npm run build`, for the production bundle.
  - The `mobile-review` skill, whose `npm run mobile:audit` fails on overflow, sub-44px or crowded tap targets, and sub-16px inputs at 375x667.
  - Review, for everything else.
- Issues: #1412, #1626

## Rule

- **Mobile-first.**
  - One control height (`components/ui/control.ts`): 44px below `sm`, 36px from `sm`. `Button` (default and `icon`), `Input`, `Select`, `Combobox`, `TogglePill`, `SegmentedTabs` and `MenuItem` all use it, so a row of controls lines up. Fields are ≥16px.
  - `Button size="sm"` and `iconSm` are 44px on phones too; they are compact only from `sm` (32px, 28px), for dense rows and tables. Don't resize a control with `h-*`, `py-*` or `text-xs`/`text-sm` (`ui/control-overrides`); a multi-line button uses `h-auto`.
  - No fixed pixel sizes, no `vh`/`h-screen` (use `dvh`), and no bare `grid-cols-N≥3`.
- **Primitives for every control and repeated pattern.** The inventory is [`web/components/ui/README.md`](../web/components/ui/README.md) (generated). If nothing fits, add a primitive.
  - `PageHeader` for the page `<h1>`. Its `actions` are all bordered: `PageHeaderLink`/`PageHeaderSettingsLink` to navigate, `secondary` buttons for commands. Only create (default) and delete (destructive) stand out.
  - `PageContainer size` for page width. The shell owns padding.
  - `Card`/`SectionCard`, and `Card variant="inset"`. Spacing comes from `padding` (`none`/`sm`/`md`), never `p-*` or `rounded-*` classes (`ui/card-padding`).
  - `SectionHeading` for every heading under `PageHeader` (`section` serif, `eyebrow` uppercase label); `SettingsSection` inside `SettingsSections` for settings pages, with no divider borders.
  - `Toolbar` for the search-and-filter row between `PageHeader` and a list.
  - `ProgressBar`, `FieldError` (with `aria-describedby`), and `RemoveButton` for an icon-only ×. `ui/shared-patterns` flags hand-built ones.
  - `useCopyFeedback` (`web/hooks/`) for a Copy → Copied button.
  - `ConfirmDialog` before any destructive action: delete, disconnect, disable, clear.
  - `LinkCard` for cards that navigate, with controls in its `actions` footer.
  - `Alert`, `LoadingState`/`ErrorState`/`EmptyState` (`placement` `page` or `inline`, no text-size classes), `Badge`, `SegmentedTabs`.
  - `Field`/`Label` for labelled controls. A field without a visible label gets `aria-label`; a placeholder is not a label.
  - `Alert tone="danger"` for errors, or `role="alert"` on an inline error beside its control, so they are announced.
  - `Dialog side="sheet"` for edits that should be phone-first.
  - `DialogDescription` goes after `DialogHeader`, or wrapped with the title in a `div`; as a direct child it lands beside the title and squeezes it (`ui/dialog-description`).
  - A hidden `<input type="file">` may use `eslint-disable-next-line no-restricted-syntax` with a reason.
- **Theme tokens only.** Colours, shadows and corner radii come from `app/globals.css`, which `ui/theme-tokens` reads. No `dark:`. Add a token rather than using a palette colour. Corners stay squared: `rounded-full` is only for square circles (`size-*`, equal `h-`/`w-`), never a pill.
- **Editorial type and motion.**
  - Display type (`PageHeader` titles, `Stat` values) is the self-hosted Fraunces serif (`font-display`); body stays sans.
  - Radii are the `rounded-xl`/`2xl` tokens (continuous-curvature feel), with no accent-ring aura on focus or hover.
  - Motion is the spring spec: `ease-spring`, `duration-press` for taps (`active:scale-[0.97]`) and `duration-settle` for entrances, with plain fades under `motion-reduce`. Reuse these tokens, not ad-hoc curves.
  - `Dialog` docks as a bottom sheet on phones with a drag handle; pull down or fling to dismiss.
  - A page's create action goes in `PageHeader` `actions`.
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
