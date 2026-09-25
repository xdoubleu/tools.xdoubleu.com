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

Static surface for grouped content. Use `interactiveCardClass` when the card is clickable.

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

Styled native checkbox; pass `label` to get the wrapping `<label>`.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `label` | `string` |  |  |

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

#### `DialogContent`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `children` | `ReactNode` | yes |  |
| `className` | `string` |  |  |
| `side` | `'center' \| 'right' \| 'fullscreen'` |  |  |

#### `DialogDescription`

#### `DialogFooter`

Right-aligned action row at the bottom of a dialog. Put the confirming action last.

#### `DialogHeader`

#### `DialogTitle`

### `input.tsx`

#### `Input`

### `label.tsx`

#### `Label`

### `menu-item.tsx`

#### `MenuItem`

### `page-container.tsx`

#### `PageContainer`

| Prop | Type | Required | Notes |
|---|---|---|---|
| `size` | `keyof typeof sizes` |  |  |

Also accepts: `HTMLAttributes<HTMLDivElement>`

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

