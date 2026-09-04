# Spec: components/ui/ primitives

<!--
GENERATED FILE — do not edit by hand.
Run `npm run generate:ui-catalog` from web/ after changing components/ui/.
Source: web/components/ui/*.tsx (JSDoc + exported prop types).
-->

- Generated from: `web/components/ui/*.tsx`
- Rule that makes these mandatory: [`convention-ui-standards.md`](convention-ui-standards.md)
- Issues: #1412

## What this is

The complete inventory of shared UI primitives. **Check here before building a
new component** — the design system's failure mode is not a missing rule, it's
not knowing what already exists. If nothing here fits, add a primitive rather
than styling a raw element at the call site; ESLint blocks the latter.

Prop tables list each component's own props. "Also accepts" means the remaining
props are forwarded to the underlying element.

## Primitives

### `CardLinkStatus.tsx` — client component

#### `CardLinkStatus`

Drop inside a navigable card's `<Link>` (which must be `relative`) to show a spinner while that link's navigation is pending — otherwise a slow route transition looks like the tap did nothing.

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

Hierarchical navigation trail. The last item is rendered as the current page (no link). Earlier items render as links when given an `href`. Use this in place of one-off "back" links so navigation looks consistent across apps.

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

Shared hover/focus treatment for clickable cards (Links or buttons rendered as cards). Apply alongside layout classes (`block`, padding, `cursor-pointer`) so every navigable card elevates the same way. Pairs with `cn()` for overrides. The accent ring is visible at rest (not just on hover/press) so clickable cards read as interactive immediately, including on touch devices with no hover state; it intensifies on `hover:`/`active:` for feedback.

### `checkbox.tsx`

#### `Checkbox`

Native `<input type="checkbox">` with the shared styling. Pass `label` to get the wrapping `<label>` too; omit it when the call site supplies its own.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `label` | `string` |  |  |

Also accepts: `Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>`

### `collapsible.tsx` — client component

#### `Collapsible`

Disclosure section with a chevron trigger. Keeps its own open state — lift it out only if something outside needs to drive it.

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
| `onChange` | `(value: string) => void` | yes | Called when the user types free text. |
| `onSelect` | `(value: string) => void` |  | Called when the user picks a suggestion (click, keyboard, or blur snap). |
| `suggestions` | `string[]` | yes |  |
| `placeholder` | `string` |  |  |
| `className` | `string` |  |  |
| `autoFocus` | `boolean` |  |  |
| `onEnter` | `() => void` |  | Called when Enter is pressed and no suggestion is highlighted. |
| `'aria-label'` | `string` |  |  |

### `date-input.tsx` — client component

#### `DateInput`

dd/MM/yyyy text field with a native date picker behind the calendar button. Native date inputs render in the browser locale (Safari shows MM/DD for en-US languages), so the visible field is a text input we format ourselves.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `value` | `string` | yes | 'YYYY-MM-DD' or '' |
| `onChange` | `(value: string) => void` | yes | Always receives 'YYYY-MM-DD' or '' — never a partial date. |
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

A lightweight popover primitive: a trigger + a portaled fixed-position panel that closes on outside-click and Escape. The panel is rendered via createPortal to document.body so it is never clipped by an ancestor overflow container (e.g. the library table's overflow-x-auto wrapper). The panel flips upward automatically when there is not enough space below the trigger, and its height is capped to the available viewport space.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `trigger` | `(props: { open: boolean; onClick: () => void }) => ReactNode` | yes |  |
| `children` | `ReactNode` | yes |  |
| `className` | `string` |  | Extra classes applied to the panel wrapper. |
| `align` | `'left' \| 'right'` |  | Alignment of the panel relative to the trigger. Defaults to "right". |

#### `PopoverTrigger`

A plain button styled for use as a popover trigger.

### `radio-group.tsx`

#### `Radio`

A bare styled radio input, for call sites that supply their own label and layout (a selectable card, a visually-hidden control behind custom visuals) and so can't use `RadioGroupItem`'s built-in label.

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

A `Card` with the standard title/description/action header already composed. Reach for this instead of assembling `Card` + `CardHeader` + `CardTitle` by hand — that hand-assembly is what drifted across the monitoring cards.

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

One labelled number in a stats row. Use for a single scalar reading; anything with its own structure belongs in a `SectionCard` instead.

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

A single on/off glyph button (favourite heart, bookmark flag). Exposes the state as `aria-pressed` so it announces as a toggle rather than an action.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `active` | `boolean` | yes | Current state — drives `aria-pressed` and the amber treatment. |
| `onToggle` | `(event: MouseEvent<HTMLButtonElement>) => void` | yes |  |
| `label` | `string` | yes | `aria-label` while off, e.g. "Add to favourites". |
| `activeLabel` | `string` | yes | `aria-label` while on, e.g. "Remove from favourites". |
| `children` | `ReactNode` | yes | The glyph — a character or an inline `<svg>`. |
| `className` | `string` |  |  |

#### `toggleGlyphClass`

Amber-when-on treatment shared by every glyph toggle (favourite, bookmark, rating star) so "this is set" reads the same everywhere.

### `toggle-pill.tsx` — client component

#### `TogglePill`

Pill control for selectable attributes (shelf, tag, ownership) and for filter chips. Filled when active, outlined when not — that contrast is what signals "this is a control", distinct from a read-only `Badge` stating a static fact. Sets `aria-pressed` by default; pass `role="tab"`/`aria-selected` instead when the pills form a tablist.

| Prop | Type | Required | Notes |
|---|---|---|---|
| `label` | `ReactNode` | yes | Pill contents. A plain string in the common case; a fragment when it carries a count. |
| `active` | `boolean` | yes |  |
| `className` | `string` |  |  |

Also accepts: `Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'>`

