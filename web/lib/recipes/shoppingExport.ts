export interface ShoppingItem {
  amount: string
  unit: string
  name: string
  id?: string
}

function formatItem(item: ShoppingItem): string {
  return `${item.amount} ${item.unit} - ${item.name}`
}

function buildSections(
  mealPlanItems: ShoppingItem[],
  customItems: ShoppingItem[],
  formatLine: (item: ShoppingItem) => string
): string {
  const sections: string[] = []
  if (mealPlanItems.length > 0) {
    sections.push(['From meal plan:', ...mealPlanItems.map(formatLine)].join('\n'))
  }
  if (customItems.length > 0) {
    sections.push(['Custom items:', ...customItems.map(formatLine)].join('\n'))
  }
  return sections.join('\n\n')
}

export function formatForClipboard(
  mealPlanItems: ShoppingItem[],
  customItems: ShoppingItem[]
): string {
  return buildSections(mealPlanItems, customItems, formatItem)
}

export function formatForAppleNotes(
  mealPlanItems: ShoppingItem[],
  customItems: ShoppingItem[],
  date: Date = new Date()
): string {
  const day = String(date.getDate()).padStart(2, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const year = date.getFullYear()
  const title = `Shopping list ${day}/${month}/${year}`
  const body = buildSections(
    mealPlanItems,
    customItems,
    (item) => `${item.amount} ${item.unit} ${item.name}`
  )
  return body ? `${title}\n\n${body}` : title
}

export function formatAsTxt(mealPlanItems: ShoppingItem[], customItems: ShoppingItem[]): string {
  return buildSections(mealPlanItems, customItems, formatItem)
}
