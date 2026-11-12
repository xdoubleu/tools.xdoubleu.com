# Convention: web UI standards

- Enforced by:
  - `npm run lint`, which runs the ESLint raw-control bans and the local `ui/*` rules in `web/eslint-rules/` (`mobile-first-classes`, `theme-tokens`, `no-classname-template`, `touch-targets`, `use-primitives`, `field-labels`, `dialog-description`, `control-overrides`, `card-padding`, `shared-patterns`), their RuleTester tests (`test:eslint-rules`) and `generate:ui-catalog:check`.
  - `npm run build`, for the production bundle.
  - The `mobile-review` skill, whose `npm run mobile:audit` fails on overflow, sub-44px or crowded tap targets, and sub-16px inputs at 375x667, and reports layout shift while loading.
  - Review, for everything else.
- Issues: #1412, #1626

## Rule

The same job uses the same primitive everywhere. The inventory is [`web/components/ui/README.md`](../web/components/ui/README.md) (generated); if nothing fits, add a primitive rather than styling a raw element.

- **One height scale** (`components/ui/control.ts`): every control is 44px below `sm` and 36px from `sm`, so a row of controls lines up. `Button size="sm"`/`iconSm` stay 44px on phones and compact only from `sm` (32px, 28px), for dense rows and tables. Don't resize a control with `h-*`, `py-*` or text-size classes; a multi-line button uses `h-auto`. Fields are ≥16px. No fixed pixel sizes, no `vh` (use `dvh`), no bare `grid-cols-N≥3`.
- **Page structure.**
  - The shell is global navigation → [adr-0050](adr-0050-global-navigation-model.md): a left sidebar from `lg` (Today, then apps by group from `web/lib/apps.ts`), below it a slim top bar with the account menu and a bottom `TabBar` (Today, three pinned apps, Apps); `--tab-bar-height` is reserved under the page, and full-screen pages (reader, presenter, shared dashboards) hide it. Settings, family, appearance and sign out live in the account menu; pages add no bar of their own.
  - `PageContainer size` sets the width; settings pages are `narrow`. The shell owns padding.
  - `PageHeader` owns the `<h1>`, the `description` line, a `breadcrumb` on every detail and settings page, and the page's `actions`. Actions are bordered (`PageHeaderLink`, `PageHeaderSettingsLink`, `secondary` buttons) at the default size; only create (default) and delete (destructive) stand out. Titles are sentence case (`ui/sentence-case` checks `title` props and `SectionHeading` text; names like Kobo or Steam are allowed), and no two pages share a title.
  - `SectionHeading` for every heading under it: `section` (semibold) or `eyebrow` (small uppercase).
  - Settings pages (`PageContainer size="narrow"`, family included) stack `SettingsSection`s in `SettingsSections`, with no dividers and default-size buttons; `ui/shared-patterns` rejects a settings page without it.
  - `Toolbar` holds the search and filters between `PageHeader` and a list, with `aria-label`led `Select`s, `TogglePill`s or `SegmentedTabs`.
  - `SegmentedTabs` switches between views; with `router.push` it switches between sibling pages.
- **One control per job.** `SegmentedTabs` (a track with a sliding segment) switches views, sibling sections and scope (Mine/Family); `TogglePill` is a light filter chip with a check when on; `Button` is an action. A `Button` or `TogglePill` with `role="tab"` is rejected (`ui/shared-patterns`). `Select` keeps the native picker with a drawn chevron.
- **Page width.** `PageContainer` caps at `max-w-5xl` by default; `size="wide"` is for tables, boards, week grids and full-height dashboards, `narrow` and `form` for forms.
- **Stable layout.** Only what the user changed moves.
  - Lists have a deterministic order (a tiebreaker on the sort key) and stable `key`s; an optimistic update sorts like the server.
  - A mutation patches the cached data and never replaces the list with a loading state.
  - Loading states reserve the final size with `Skeleton`; nothing is inserted above existing content after first paint.
- **Surfaces.** `Card`/`SectionCard`/`Card variant="inset"` take spacing from `padding` (`none`/`sm`/`md`), never `p-*` or `rounded-*` classes. `LinkCard` for cards that navigate, with controls in its `actions` footer. Cards have a fill and a soft shadow, no border (inputs keep theirs). A list of single-line items is one `GroupedList` of `GroupedListItem`s, not a stack of cards.
- **States.** `LoadingState`/`ErrorState`/`EmptyState` with `placement` (`page` for a whole page, `inline` inside a card, panel or dialog), never hand-rolled centred paragraphs or text-size classes.
- **Collections.** A list page is a `CollectionToolbar` (search that filters, facet `TogglePill`s, count), a `CollectionGrid` of `ItemCard`s and an `EmptyState` with an icon, one sentence and one primary action. A hand-written grid in a `*ListClient.tsx` is rejected (`ui/shared-patterns`). Adding from a catalogue is a separate control, never the list search.
- **App sections.** An app with several top-level pages (Library and Stats, Backlog and Stats, Inbox and Stats) shows `AppSections`, one `SegmentedTabs` under the title, from the `sections` of its `web/lib/apps.ts` entry. Apps open on their collection; stats are a section, not the front door.
- **Detail pages.** `DetailHeader` holds breadcrumb, artwork, title, facts, one status control, a favourite slot and bordered actions such as Edit. Status with up to five values is `SegmentedTabs`, more is a `Select`. Delete or remove is an entry in the header's overflow menu that opens a `ConfirmDialog`, never a red button on the page.
- **Shared patterns** (`ui/shared-patterns`): `ProgressBar`; `FieldError` for an error under a control (with `aria-describedby`), `Alert tone="danger"` for a form-level one; `RemoveButton` for an icon-only ×; `useCopyFeedback` (`web/hooks/`) for Copy → Copied.
- **One height per row.** Controls sharing a row share a size; Refresh is a `secondary` button at its neighbours' size. `sm` is only for dense rows or tables, never beside default-size controls.
- **Create in `PageHeader` actions, edit in a sheet.** `Dialog` defaults to `side="sheet"`, a bottom sheet on phones with a drag handle. `DialogDescription` goes after `DialogHeader` or wrapped with the title.
- **`ConfirmDialog` before every destructive action**: delete, disconnect, disable, leave, clear.
- **Labels.** `Field`/`Label` for labelled controls; a control without a visible label gets `aria-label` (a placeholder is not a label).
- **Theme tokens only.** Colours, shadows and radii come from `app/globals.css` (`ui/theme-tokens`). No `dark:`. `rounded-full` only for square circles, never a pill.
- **Type and motion.** One system-font stack throughout (`ui/theme-tokens` rejects `font-display`/`font-serif`); headings differ by size and weight only (`PageHeader` bold large title, `SectionHeading`, `StatTile` semibold). Motion uses the `ease-spring`, `duration-press` and `duration-settle` tokens, with plain fades under `motion-reduce`.
- **Values print through `lib/dates.ts` and `lib/format.ts`** (`ui/no-tolocale`): short dates ("11 Nov", year only outside the current one), times without seconds, numbers with at most one decimal, `formatQuantity` (no amount when zero), `pluralize`. No `toLocale*` elsewhere.
- Class overrides go through `cn()`, never a template literal. Pending labels end in `…`, never `...`. A hidden `<input type="file">` may disable `no-restricted-syntax` with a reason.
- **Pinch-zoom is disabled** (the viewport meta in `web/index.html`), which makes 16px fields and 44px targets mandatory.

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
