export interface ShoppingItem {
  amount: string
  unit: string
  name: string
  id?: string
}

export interface DayItems {
  date: string
  items: ShoppingItem[]
}

function formatItem(item: ShoppingItem): string {
  return `${item.amount} ${item.unit} - ${item.name}`
}

function buildSections(
  customItems: ShoppingItem[],
  dayItems: DayItems[] | undefined,
  formatLine: (item: ShoppingItem) => string
): string {
  const sections: string[] = []
  if (customItems.length > 0) {
    sections.push(['Custom items:', ...customItems.map(formatLine)].join('\n'))
  }
  if (dayItems && dayItems.length > 0) {
    const dayLines: string[] = ['From meal plan:']
    for (const day of dayItems) {
      dayLines.push(day.date + ':')
      for (const item of day.items) {
        dayLines.push('  ' + formatLine(item))
      }
    }
    sections.push(dayLines.join('\n'))
  }
  return sections.join('\n\n')
}

export function formatForClipboard(customItems: ShoppingItem[], dayItems?: DayItems[]): string {
  return buildSections(customItems, dayItems, formatItem)
}

export function formatForAppleNotes(
  customItems: ShoppingItem[],
  dayItems?: DayItems[],
  date: Date = new Date()
): string {
  const day = String(date.getDate()).padStart(2, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const year = date.getFullYear()
  const title = `Shopping list ${day}/${month}/${year}`
  const body = buildSections(
    customItems,
    dayItems,
    (item) => `${item.amount} ${item.unit} ${item.name}`
  )
  return body ? `${title}\n\n${body}` : title
}

export function formatAsTxt(customItems: ShoppingItem[], dayItems?: DayItems[]): string {
  return buildSections(customItems, dayItems, formatItem)
}
