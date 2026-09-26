# components/ui/

<!--
GENERATED FILE — do not edit by hand.
Run `npm run generate:ui-catalog` from web/ after changing components/ui/.
Source: web/components/ui/*.tsx (JSDoc + exported prop types).
-->

Every shared UI primitive. **Check here before building a component**; if
nothing fits, add a primitive here — ESLint blocks styling raw elements at call
sites ([`docs/convention-ui-standards.md`](../../../docs/convention-ui-standards.md)).

"Also accepts" props are forwarded to the underlying element.

## Primitives

### `CardLinkStatus.tsx` — client component

#### `CardLinkStatus`

Spinner inside a card's (relative) `<Link>` while its navigation is pending.

### `LoadMoreButton.tsx`

#### `LoadMoreButton`

### `PageLoading.tsx`

#### `PageLoading`

### `alert.tsx`

#### `Alert`

Inline message banner. `danger` announces immediately (`role="alert"`); the other tones are polite (`role="status"`).

| Prop | Type | Required | Notes |
|---|---|---|---|
| `tone` | `'danger' \| 'success' \| 'warn' \| 'info'` |  |  |

Also accepts: `HTMLAttributes<HTMLDivElement>`

### `badge.tsx`

#### `Badge`

Small inline status pill. Pick the variant by meaning, not colour.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `variant` | `'default' \| 'secondary' \| 'success' \| 'warn' \| 'danger'` |  |  |

Also accepts: `HTMLAttributes<HTMLSpanElement>`

### `breadcrumb.tsx`

#### `Breadcrumb`

Navigation trail; the last item is the current page. Use instead of one-off "back" links.

### `button.tsx`

#### `Button`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `variant` | `'default' \| 'secondary' \| 'ghost' \| 'destructive' \| 'link'` |  |  |
| `size` | `'sm' \| 'md' \| 'lg' \| 'icon' \| 'iconSm'` |  |  |
| `asChild` | `boolean` |  | Render the child element as the button (e.g. a Next `<Link>`), inheriting button styles. |

Also accepts: `ButtonHTMLAttributes<HTMLButtonElement>`

### `card.tsx`

#### `Card`

Static surface for grouped content. Use `LinkCard` when the card navigates, `interactiveCardClass` for other clickable cards.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `variant` | `keyof typeof cardVariants` |  | `inset` is a flat, padded panel nested inside a card or dialog (list rows, notes). |

Also accepts: `HTMLAttributes<HTMLDivElement>`

#### `CardContent`

Main body of a `Card`, padded to line up with `CardHeader`.

#### `CardDescription`

Muted supporting line inside a `CardHeader`.

#### `CardFooter`

Action row at the bottom of a `Card`.

#### `CardHeader`

Title/description block at the top of a `Card`.

#### `CardTitle`

Heading inside a `CardHeader`.

#### `interactiveCardClass`

Hover/focus treatment for clickable cards. The accent ring shows at rest so cards read as interactive on touch; it intensifies on hover/active.

### `checkbox.tsx`

#### `Checkbox`

Styled native checkbox; pass `label` to get the wrapping `<label>` (a bare one is a 16px target).

| Prop | Type | Required | Notes |
|---|---|---|---|
| `label` | `ReactNode` |  | Rendered in a 44px-tall wrapping `<label>`; wrap text in `sr-only` to hide it visually. |
| `labelClassName` | `string` |  |  |

Also accepts: `Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>`

### `collapsible.tsx` — client component

#### `Collapsible`

Disclosure section with a chevron trigger and its own open state.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `title` | `ReactNode` | yes |  |
| `defaultCollapsed` | `boolean` |  |  |
| `children` | `ReactNode` | yes |  |
| `className` | `string` |  |  |
| `triggerClassName` | `string` |  | Applied to the trigger, e.g. to change its type scale. |

### `combobox.tsx` — client component

#### `Combobox`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `value` | `string` | yes |  |
| `onChange` | `(value: string) => void` | yes |  |
| `onSelect` | `(value: string) => void` |  | Called when the user picks a suggestion (click, keyboard, or blur snap). |
| `suggestions` | `string[]` | yes |  |
| `placeholder` | `string` |  |  |
| `className` | `string` |  |  |
| `autoFocus` | `boolean` |  |  |
| `onEnter` | `() => void` |  | Called when Enter is pressed and no suggestion is highlighted. |
| `'aria-label'` | `string` |  |  |

### `date-input.tsx` — client component

#### `DateInput`

Native date input on every viewport; its value is always 'YYYY-MM-DD' or ''.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `value` | `string` | yes | 'YYYY-MM-DD' or '' |
| `onChange` | `(value: string) => void` | yes |  |
| `onBlur` | `() => void` |  |  |
| `id` | `string` |  |  |
| `className` | `string` |  |  |
| `'aria-label'` | `string` |  |  |

### `dialog.tsx` — client component

#### `ConfirmDialog`

Confirmation prompt for an irreversible action. Prefer this over composing `Dialog` by hand so every confirm step reads and behaves the same.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `open` | `boolean` | yes |  |
| `onOpenChange` | `(open: boolean) => void` | yes |  |
| `title` | `ReactNode` | yes |  |
| `description` | `ReactNode` |  | Say what will happen, especially what cannot be undone. |
| `confirmLabel` | `string` |  |  |
| `pendingLabel` | `string` |  | Shown on the confirm button while `pending` — a `…`-suffixed present participle. |
| `cancelLabel` | `string` |  |  |
| `destructive` | `boolean` |  | Styles the confirm action as destructive. |
| `pending` | `boolean` |  |  |
| `confirmDisabled` | `boolean` |  | Blocks confirming while required input in `children` is missing. |
| `onConfirm` | `() => void` | yes |  |
| `children` | `ReactNode` |  |  |

#### `Dialog`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `open` | `boolean` | yes |  |
| `onOpenChange` | `(open: boolean) => void` | yes |  |
| `modal` | `boolean` |  |  |
| `children` | `ReactNode` | yes |  |

#### `DialogClose`

44px close control; defaults to a `×` glyph labelled "Close".

#### `DialogContent`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `children` | `ReactNode` | yes |  |
| `className` | `string` |  |  |
| `side` | `'center' \| 'right' \| 'fullscreen' \| 'sheet'` |  | `sheet` is a bottom sheet on phones and a centered dialog from `sm` up. |

#### `DialogDescription`

#### `DialogFooter`

Right-aligned action row, pinned to the bottom of a scrolling dialog so its actions stay reachable. Put the confirming action last.

#### `DialogHeader`

#### `DialogTitle`

### `field.tsx`

#### `Field`

A form control with its label and hint/error, stacked.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `label` | `ReactNode` | yes |  |
| `htmlFor` | `string` | yes | The control's `id`, so tapping the label focuses it. |
| `hint` | `ReactNode` |  | Muted help text under the control. |
| `error` | `ReactNode` |  | Validation message; replaces `hint` while set. |
| `children` | `ReactNode` | yes |  |
| `className` | `string` |  |  |

### `input.tsx`

#### `Input`

### `label.tsx`

#### `Label`

### `link-card.tsx`

#### `LinkCard`

A card that navigates, with its controls kept out of the link area.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `href` | `string` | yes |  |
| `children` | `ReactNode` | yes | The navigating zone: cover, title, meta. Keep controls out of it. |
| `actions` | `ReactNode` |  | Controls in a separate footer that never navigates, so a near-miss can't open the page. |
| `'aria-label'` | `string` |  | Accessible name for the link when `children` has no clear text. |
| `className` | `string` |  |  |
| `linkClassName` | `string` |  |  |

### `menu-item.tsx`

#### `MenuItem`

### `page-container.tsx`

#### `PageContainer`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `size` | `keyof typeof sizes` |  |  |

Also accepts: `HTMLAttributes<HTMLDivElement>`

### `page-header.tsx`

#### `PageHeader`

The page's `<h1>` row. Every page uses this instead of a hand-styled heading.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `title` | `ReactNode` | yes |  |
| `breadcrumb` | `BreadcrumbItem[]` |  | Trail above the title; the last item is the current page. |
| `description` | `ReactNode` |  | Muted line under the title. |
| `actions` | `ReactNode` |  | Page-level controls; they wrap under the title on narrow screens. |
| `className` | `string` |  |  |

### `popover.tsx` — client component

#### `Popover`

Trigger plus a portalled fixed panel (never clipped by overflow ancestors) that closes on outside click and Escape, flips upward when space below is short, and caps its height to the viewport.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `trigger` | `(props: { open: boolean; onClick: () => void }) => ReactNode` | yes |  |
| `children` | `ReactNode` | yes |  |
| `className` | `string` |  |  |
| `align` | `'left' \| 'right'` |  | Alignment of the panel relative to the trigger. Defaults to "right". |

#### `PopoverTrigger`

A plain button styled for use as a popover trigger.

### `radio-group.tsx`

#### `Radio`

A bare styled radio for call sites that supply their own label and layout.

#### `RadioGroup`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `name` | `string` | yes |  |
| `value` | `string` | yes |  |
| `onChange` | `(value: string) => void` | yes |  |

Also accepts: `Omit<HTMLAttributes<HTMLDivElement>, 'onChange'>`

#### `RadioGroupItem`

One labelled radio inside a `RadioGroup`. Use `Radio` for a custom layout.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `value` | `string` | yes |  |
| `label` | `string` | yes |  |

Also accepts: `Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'type' | 'name' | 'checked' | 'onChange'
>`

### `section-card.tsx`

#### `SectionCard`

A `Card` with the standard title/description/action header; don't hand-assemble one.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `title` | `ReactNode` | yes |  |
| `description` | `ReactNode` |  | Muted line under the title. |
| `action` | `ReactNode` |  | Right-aligned controls on the title row (a refresh button, a filter, a count). |
| `children` | `ReactNode` | yes |  |
| `className` | `string` |  |  |
| `contentClassName` | `string` |  | Applied to the body wrapper, not the card. |

### `segmented-tabs.tsx` — client component

#### `SegmentedTabs`

Single-choice switch between a few views or modes, rendered as a tablist.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `value` | `T` | yes |  |
| `onChange` | `(value: T) => void` | yes |  |
| `options` | `SegmentedTabsOption<T>[]` | yes |  |
| `'aria-label'` | `string` | yes | Names the tablist for screen readers. |
| `className` | `string` |  |  |

### `select.tsx`

#### `Select`

### `stat.tsx`

#### `StatTile`

One labelled number in a stats row; structured content belongs in a `SectionCard`.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `label` | `string` | yes |  |
| `value` | `ReactNode` | yes |  |
| `tone` | `'default' \| 'success' \| 'warn' \| 'danger'` |  | Colours the value only — the label stays muted so tiles scan as one row. |
| `hint` | `ReactNode` |  | Optional muted line under the value (a delta, a unit, a timestamp). |
| `href` | `string` |  | Renders the tile as a navigable card with a pending-navigation spinner. |
| `className` | `string` |  |  |

#### `StatTileGrid`

Responsive grid for a row of `StatTile`s — two up on mobile, four from `sm`.

### `states.tsx`

#### `EmptyState`

Message for a list or section with nothing in it yet.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `children` | `ReactNode` | yes |  |
| `action` | `ReactNode` |  | Optional call to action under the message, e.g. a "New recipe" button. |
| `className` | `string` |  |  |

#### `ErrorState`

Fetch failure message: "Failed to load {what}."

#### `LoadingState`

Loading placeholder; `label` names what is loading ("recipe" → "Loading recipe…").

### `table.tsx`

#### `SortableHeader`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `dir` | `'asc' \| 'desc' \| null` | yes |  |
| `onSort` | `() => void` | yes |  |

Also accepts: `Omit<ThHTMLAttributes<HTMLTableCellElement>, 'dir'>`

#### `Table`

#### `TableBody`

#### `TableCell`

#### `TableHead`

#### `TableHeader`

#### `TableRow`

### `textarea.tsx`

#### `Textarea`

### `toggle-icon-button.tsx` — client component

#### `ToggleIconButton`

On/off glyph button; `aria-pressed` makes it announce as a toggle.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `active` | `boolean` | yes | Current state — drives `aria-pressed` and the amber treatment. |
| `onToggle` | `(event: MouseEvent<HTMLButtonElement>) => void` | yes |  |
| `label` | `string` | yes | `aria-label` while off, e.g. "Add to favourites". |
| `activeLabel` | `string` | yes | `aria-label` while on, e.g. "Remove from favourites". |
| `children` | `ReactNode` | yes |  |
| `className` | `string` |  |  |

#### `toggleGlyphClass`

Amber-when-on treatment shared by every glyph toggle.

### `toggle-pill.tsx` — client component

#### `TogglePill`

Pill for selectable attributes and filter chips: filled when active, outlined when not (unlike a read-only `Badge`). Sets `aria-pressed`; pass `role="tab"`/`aria-selected` in a tablist.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `label` | `ReactNode` | yes | Pill contents. A plain string in the common case; a fragment when it carries a count. |
| `active` | `boolean` | yes |  |
| `className` | `string` |  |  |

Also accepts: `Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'>`

