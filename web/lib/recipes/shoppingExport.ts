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
  customItems: ShoppingItem[],
  mealItems: ShoppingItem[] | undefined,
  formatLine: (item: ShoppingItem) => string
): string {
  const sections: string[] = []
  if (customItems.length > 0) {
    sections.push(['Custom items:', ...customItems.map(formatLine)].join('\n'))
  }
  if (mealItems && mealItems.length > 0) {
    const lines = ['From meal plan:', ...mealItems.map((item) => '  ' + formatLine(item))]
    sections.push(lines.join('\n'))
  }
  return sections.join('\n\n')
}

export function formatForClipboard(
  customItems: ShoppingItem[],
  mealItems?: ShoppingItem[]
): string {
  return buildSections(customItems, mealItems, formatItem)
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
  const body = buildSections(
    customItems,
    mealItems,
    (item) => `${item.amount} ${item.unit} ${item.name}`
  )
  return body ? `${title}\n\n${body}` : title
}

export function formatAsTxt(customItems: ShoppingItem[], mealItems?: ShoppingItem[]): string {
  return buildSections(customItems, mealItems, formatItem)
}
