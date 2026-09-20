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

function upgradeUnit(item: ShoppingItem): ShoppingItem {
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

function formatOriginLabel(o: ItemOrigin): string {
  return o.groupName ? `${o.recipeName} [${o.groupName}]` : o.recipeName
}

export function formatOrigins(origins: ItemOrigin[] | undefined): string {
  if (!origins || origins.length === 0) return ''
  if (origins.length === 1) return ` (${formatOriginLabel(origins[0])})`
  return ` (${origins.map((o) => `${formatOriginLabel(o)}: ${o.amount} ${o.unit}`).join(', ')})`
}

function normalizeName(name: string): string {
  return name.trim().toLowerCase()
}

// Combines meal plan items (those with recipeName) that share the same
// normalized name and unit into a single item with summed amounts and origins.
// Custom items (no recipeName) pass through unchanged.
function combineItems(items: ShoppingItem[]): ShoppingItem[] {
  const result: ShoppingItem[] = []
  const mealByKey = new Map<string, ShoppingItem[]>()

  for (const item of items) {
    if (!item.recipeName) {
      result.push(item)
      continue
    }
    const key = `${normalizeName(item.name)}::${item.unit}`
    const group = mealByKey.get(key)
    if (group) group.push(item)
    else mealByKey.set(key, [item])
  }

  for (const group of mealByKey.values()) {
    const first = group[0]
    const origins: ItemOrigin[] = group.map((i) => ({
      recipeName: i.recipeName!,
      amount: i.amount,
      unit: i.unit,
      groupName: i.groupName
    }))
    if (group.length === 1) {
      result.push({ ...first, origins })
    } else {
      const allNumeric = group.every((i) => !isNaN(parseFloat(i.amount)))
      const amount = allNumeric
        ? formatAmount(group.reduce((sum, i) => sum + parseFloat(i.amount), 0))
        : first.amount
      result.push({ name: first.name, amount, unit: first.unit, origins })
    }
  }

  return result
}

// Merges custom and meal plan items, combines meal items that share the same
// ingredient name and unit across recipes, then upgrades units. Combining runs
// on the original source units (e.g. grams): upgrading first would convert a
// large amount (1500 g → 1.5 kg, common for bulk-prep recipes) while a smaller
// amount of the same ingredient stays in grams, so the units would diverge and
// the two lines would never combine.
function mergeItems(
  customItems: ShoppingItem[],
  mealItems: ShoppingItem[] | undefined
): ShoppingItem[] {
  return combineItems([...customItems, ...(mealItems ?? [])]).map(upgradeUnit)
}

// Returns the fully prepared (merged, combined, unit-upgraded) item list for
// the given custom and meal plan items. Exported for use in the export preview.
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

// Items that can't be placed in the store's walk-through order are collected
// under this trailing bucket in the exported list.
const OTHER_CATEGORY = 'Other'

// The outcome of bucketing items against a store's ordered categories. `groups`
// holds the items that map to a category present in this store, in walk-through
// order. The remaining items can't be ordered and are split so the UI can warn
// about each case distinctly:
//   - `uncategorized`: the item's name maps to no category at all.
//   - `unordered`: the item has a category, but it isn't part of this store's
//     ordering, so we can't tell where it falls in the walk-through.
export interface StoreGrouping {
  groups: CategoryGroup[]
  uncategorized: ShoppingItem[]
  unordered: ShoppingItem[]
}

// groupByStore merges the custom and meal items, then buckets them by the
// category their (normalized) name maps to, emitting the buckets in the store's
// walk-through order. Categories with no items are omitted. Items that can't be
// placed are returned separately, split by whether they lack a category
// entirely or carry one that this store doesn't order.
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

// toExportGroups flattens a StoreGrouping into the list passed to the grouped
// formatters: the store-ordered groups followed by a single trailing "Other"
// group holding every item that couldn't be ordered (both uncategorized and
// not-ordered-by-this-store items).
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

// Apple Notes is exported as a flat list without store category headers: Notes
// renders better as a single checklist, so the store ordering is preserved only
// in the order of the items, not as section titles.
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
