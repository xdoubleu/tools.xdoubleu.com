export interface ShoppingItem {
  amount: string
  unit: string
  name: string
}

export function formatForClipboard(items: ShoppingItem[]): string {
  return items.map((item) => `${item.amount} ${item.unit} - ${item.name}`).join('\n')
}

export function formatForAppleNotes(items: ShoppingItem[]): string {
  return items.map((item) => `[ ] ${item.amount} ${item.unit} ${item.name}`).join('\n')
}

export function formatAsTxt(items: ShoppingItem[]): string {
  return items.map((item) => `${item.amount} ${item.unit} - ${item.name}`).join('\n')
}
