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

export function formatForAppleNotes(
  customItems: ShoppingItem[],
  mealItems?: ShoppingItem[],
  date: Date = new Date()
): string {
  const day = String(date.getDate()).padStart(2, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const year = date.getFullYear()
  const title = `Shopping list ${day}/${month}/${year}`
  const body = mergeItems(customItems, mealItems)
    .map((item) => `${item.amount} ${item.unit} ${item.name}`)
    .join('\n')
  return body ? `${title}\n\n${body}` : title
}

export function formatAsTxt(customItems: ShoppingItem[], mealItems?: ShoppingItem[]): string {
  return formatForClipboard(customItems, mealItems)
}
