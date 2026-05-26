export interface ShoppingItem {
  amount: string
  unit: string
  name: string
}

export function formatForClipboard(items: ShoppingItem[]): string {
  return items.map((item) => `${item.amount} ${item.unit} - ${item.name}`).join('\n')
}

export function formatForAppleNotes(items: ShoppingItem[], date: Date = new Date()): string {
  const day = String(date.getDate()).padStart(2, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const year = date.getFullYear()
  const title = `Shopping list ${day}/${month}/${year}`
  const lines = items.map((item) => `${item.amount} ${item.unit} ${item.name}`)
  return [title, ...lines].join('\n')
}

export function formatAsTxt(items: ShoppingItem[]): string {
  return items.map((item) => `${item.amount} ${item.unit} - ${item.name}`).join('\n')
}
