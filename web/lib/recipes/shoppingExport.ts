export interface ShoppingItem {
  amount: string
  unit: string
  name: string
  id?: string
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

function mergeItems(
  customItems: ShoppingItem[],
  mealItems: ShoppingItem[] | undefined
): ShoppingItem[] {
  return [...customItems, ...(mealItems ?? [])].map(upgradeUnit)
}

export function formatForClipboard(
  customItems: ShoppingItem[],
  mealItems?: ShoppingItem[]
): string {
  return mergeItems(customItems, mealItems)
    .map((item) => `${item.amount} ${item.unit} - ${item.name}`)
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
    .map((item) => `${item.amount} ${item.unit} ${item.name}`)
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

// Items whose name maps to no category, or to one not present in the chosen
// store's ordering, are collected under this trailing bucket.
export const OTHER_CATEGORY = 'Other'

function normalizeName(name: string): string {
  return name.trim().toLowerCase()
}

// groupByStore merges the custom and meal items, then buckets them by the
// category their (normalized) name maps to, emitting the buckets in the store's
// walk-through order. Categories with no items are omitted; unmapped items land
// in a trailing "Other" group.
export function groupByStore(
  customItems: ShoppingItem[],
  mealItems: ShoppingItem[] | undefined,
  orderedCategories: Category[],
  nameToCategoryId: Record<string, string>
): CategoryGroup[] {
  const merged = mergeItems(customItems, mealItems)

  const byCategoryId = new Map<string, ShoppingItem[]>()
  for (const category of orderedCategories) {
    byCategoryId.set(category.id, [])
  }
  const other: ShoppingItem[] = []

  for (const item of merged) {
    const categoryId = nameToCategoryId[normalizeName(item.name)]
    const bucket = categoryId ? byCategoryId.get(categoryId) : undefined
    if (bucket) bucket.push(item)
    else other.push(item)
  }

  const groups: CategoryGroup[] = []
  for (const category of orderedCategories) {
    const items = byCategoryId.get(category.id) ?? []
    if (items.length > 0) groups.push({ category: category.name, items })
  }
  if (other.length > 0) groups.push({ category: OTHER_CATEGORY, items: other })
  return groups
}

export function formatGroupedForClipboard(groups: CategoryGroup[]): string {
  return groups
    .map((group) =>
      [
        `${group.category}:`,
        ...group.items.map((item) => `${item.amount} ${item.unit} - ${item.name}`)
      ].join('\n')
    )
    .join('\n\n')
}

export function formatGroupedAsTxt(groups: CategoryGroup[]): string {
  return formatGroupedForClipboard(groups)
}

export function formatGroupedForAppleNotes(
  groups: CategoryGroup[],
  date: Date = new Date()
): string {
  const title = appleNotesTitle(date)
  const body = groups
    .map((group) =>
      [
        `${group.category}:`,
        ...group.items.map((item) => `${item.amount} ${item.unit} ${item.name}`)
      ].join('\n')
    )
    .join('\n\n')
  return body ? `${title}\n\n${body}` : title
}
