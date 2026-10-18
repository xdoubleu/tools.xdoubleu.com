export interface ItemOrigin {
  recipeName: string
  amount: string
  unit: string
  groupName?: string
}

export interface ShoppingItem {
  amount: string
  unit: string
  name: string
  id?: string
  recipeName?: string
  groupName?: string
  origins?: ItemOrigin[]
}

const UNIT_UPGRADES: Record<string, { threshold: number; nextUnit: string; divisor: number }> = {
  g: { threshold: 1000, nextUnit: 'kg', divisor: 1000 },
  ml: { threshold: 1000, nextUnit: 'L', divisor: 1000 },
  mg: { threshold: 1000, nextUnit: 'g', divisor: 1000 }
}

function upgradeUnit<T extends { amount: string; unit: string }>(item: T): T {
  const upgrade = UNIT_UPGRADES[item.unit]
  if (!upgrade) return item
  const amount = parseFloat(item.amount)
  if (isNaN(amount) || amount < upgrade.threshold) return item
  const upgraded = amount / upgrade.divisor
  const formatted = upgraded % 1 === 0 ? String(upgraded) : String(parseFloat(upgraded.toFixed(3)))
  return { ...item, amount: formatted, unit: upgrade.nextUnit }
}

function formatAmount(n: number): string {
  return n % 1 === 0 ? String(n) : String(parseFloat(n.toFixed(3)))
}

const CUSTOM_ORIGIN = 'Custom'

function formatOriginLabel(o: ItemOrigin): string {
  return o.groupName ? `${o.recipeName} [${o.groupName}]` : o.recipeName
}

// A bare (zero-amount) origin shows only its label.
function formatOriginEntry(o: ItemOrigin): string {
  const quantity = parseFloat(o.amount) === 0 ? '' : `${o.amount} ${o.unit}`.trim()
  return quantity ? `${formatOriginLabel(o)}: ${quantity}` : formatOriginLabel(o)
}

export function formatOrigins(origins: ItemOrigin[] | undefined): string {
  if (!origins || origins.length === 0) return ''
  if (origins.length === 1) return ` (${formatOriginLabel(origins[0])})`
  return ` (${origins.map(formatOriginEntry).join(', ')})`
}

function normalizeName(name: string): string {
  return name.trim().toLowerCase()
}

// Non-numeric amounts fall back to the first.
function sumAmounts(items: ShoppingItem[]): string {
  return items.every((i) => !isNaN(parseFloat(i.amount)))
    ? formatAmount(items.reduce((sum, i) => sum + parseFloat(i.amount), 0))
    : items[0].amount
}

function groupBy(items: ShoppingItem[], key: (item: ShoppingItem) => string) {
  const groups = new Map<string, ShoppingItem[]>()
  for (const item of items) {
    const k = key(item)
    const group = groups.get(k)
    if (group) group.push(item)
    else groups.set(k, [item])
  }
  return [...groups.values()]
}

// Sums each unit, upgrading after summing so 1500 g and 200 g still merge
// before becoming kg. Mixed units join into amount, leaving the last unit in
// `unit`, so `${amount} ${unit}` reads "200 g + 1 stuk". Zero-amount parts
// (bare entries) drop out next to specified ones.
function combineGroup(group: ShoppingItem[]): ShoppingItem {
  const parts = groupBy(group, (i) => i.unit).map((items) =>
    upgradeUnit({ amount: sumAmounts(items), unit: items[0].unit })
  )
  const specified = parts.filter((p) => parseFloat(p.amount) !== 0)
  const shown = specified.length > 0 ? specified : parts
  const last = shown[shown.length - 1]
  const amount = [
    ...shown.slice(0, -1).map((p) => `${p.amount} ${p.unit}`.trim()),
    last.amount
  ].join(' + ')

  const origins = group.some((i) => i.recipeName)
    ? group.map((i) => ({
        recipeName: i.recipeName || CUSTOM_ORIGIN,
        amount: i.amount,
        unit: i.unit,
        groupName: i.groupName
      }))
    : undefined
  if (group.length === 1) return { ...group[0], amount, unit: last.unit, origins }
  return { name: group[0].name, amount, unit: last.unit, origins }
}

// One line per normalized name, so custom and meal-plan items combine.
function combineItems(items: ShoppingItem[]): ShoppingItem[] {
  return groupBy(items, (i) => normalizeName(i.name)).map(combineGroup)
}

function mergeItems(
  customItems: ShoppingItem[],
  mealItems: ShoppingItem[] | undefined
): ShoppingItem[] {
  return combineItems([...customItems, ...(mealItems ?? [])])
}

// Merged, combined, unit-upgraded list; also used by the export preview.
export function prepareForExport(
  customItems: ShoppingItem[],
  mealItems?: ShoppingItem[]
): ShoppingItem[] {
  return mergeItems(customItems, mealItems)
}

export function formatForClipboard(
  customItems: ShoppingItem[],
  mealItems?: ShoppingItem[]
): string {
  return mergeItems(customItems, mealItems)
    .map((item) => `${item.amount} ${item.unit} - ${item.name}${formatOrigins(item.origins)}`)
    .join('\n')
}

function appleNotesTitle(date: Date): string {
  const day = String(date.getDate()).padStart(2, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const year = date.getFullYear()
  return `Shopping list ${day}/${month}/${year}`
}

export function formatForAppleNotes(
  customItems: ShoppingItem[],
  mealItems?: ShoppingItem[],
  date: Date = new Date()
): string {
  const title = appleNotesTitle(date)
  const body = mergeItems(customItems, mealItems)
    .map((item) => `${item.amount} ${item.unit} ${item.name}${formatOrigins(item.origins)}`)
    .join('\n')
  return body ? `${title}\n\n${body}` : title
}

export function formatAsTxt(customItems: ShoppingItem[], mealItems?: ShoppingItem[]): string {
  return formatForClipboard(customItems, mealItems)
}

// ── Store-ordered grouping ────────────────────────────────────────────────────

export interface Category {
  id: string
  name: string
}

export interface CategoryGroup {
  category: string
  items: ShoppingItem[]
}

const OTHER_CATEGORY = 'Other'

// Items bucketed by store walk-through order. Leftovers are split so the UI
// can warn separately: `uncategorized` maps to no category, `unordered` has
// one this store doesn't order.
export interface StoreGrouping {
  groups: CategoryGroup[]
  uncategorized: ShoppingItem[]
  unordered: ShoppingItem[]
}

// groupByStore buckets merged items by category in the store's order,
// omitting empty categories.
export function groupByStore(
  customItems: ShoppingItem[],
  mealItems: ShoppingItem[] | undefined,
  orderedCategories: Category[],
  nameToCategoryId: Record<string, string>
): StoreGrouping {
  const merged = mergeItems(customItems, mealItems)

  const byCategoryId = new Map<string, ShoppingItem[]>()
  for (const category of orderedCategories) {
    byCategoryId.set(category.id, [])
  }
  const uncategorized: ShoppingItem[] = []
  const unordered: ShoppingItem[] = []

  for (const item of merged) {
    const categoryId = nameToCategoryId[normalizeName(item.name)]
    if (!categoryId) {
      uncategorized.push(item)
      continue
    }
    const bucket = byCategoryId.get(categoryId)
    if (bucket) bucket.push(item)
    else unordered.push(item)
  }

  const groups: CategoryGroup[] = []
  for (const category of orderedCategories) {
    const items = byCategoryId.get(category.id) ?? []
    if (items.length > 0) groups.push({ category: category.name, items })
  }
  return { groups, uncategorized, unordered }
}

// toExportGroups appends one trailing "Other" group for all leftovers.
export function toExportGroups(grouping: StoreGrouping): CategoryGroup[] {
  const other = [...grouping.uncategorized, ...grouping.unordered]
  if (other.length === 0) return grouping.groups
  return [...grouping.groups, { category: OTHER_CATEGORY, items: other }]
}

export function formatGroupedForClipboard(groups: CategoryGroup[]): string {
  return groups
    .map((group) =>
      [
        `${group.category}:`,
        ...group.items.map(
          (item) => `${item.amount} ${item.unit} - ${item.name}${formatOrigins(item.origins)}`
        )
      ].join('\n')
    )
    .join('\n\n')
}

export function formatGroupedAsTxt(groups: CategoryGroup[]): string {
  return formatGroupedForClipboard(groups)
}

// Apple Notes gets a flat checklist; store order survives only as item order.
export function formatGroupedForAppleNotes(
  groups: CategoryGroup[],
  date: Date = new Date()
): string {
  const title = appleNotesTitle(date)
  const body = groups
    .flatMap((group) => group.items)
    .map((item) => `${item.amount} ${item.unit} ${item.name}${formatOrigins(item.origins)}`)
    .join('\n')
  return body ? `${title}\n\n${body}` : title
}
