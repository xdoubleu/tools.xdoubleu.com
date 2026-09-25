// Custom meal entries store items newline-separated in `customName`, each
// `name[<TAB>amount[<TAB>unit]]` (tab can't be typed in a text input).

export interface CustomItem {
  name: string
  amount: string
  unit?: string
  // UI-only: saved to the name->category catalog, never encoded.
  categoryId?: string
}

const SEP = '\t'

export function parseCustomItems(customName: string): CustomItem[] {
  return customName
    .split('\n')
    .filter(Boolean)
    .map((line) => {
      const parts = line.split(SEP)
      if (parts.length === 1) return { name: parts[0], amount: '' }
      if (parts.length === 2) return { name: parts[0], amount: parts[1] }
      return { name: parts[0], amount: parts[1], unit: parts[2] }
    })
}

export function encodeCustomItems(items: CustomItem[]): string {
  return items
    .map((it) => ({ name: it.name.trim(), amount: it.amount.trim(), unit: (it.unit ?? '').trim() }))
    .filter((it) => it.name)
    .map((it) => {
      if (!it.amount) return it.name
      if (!it.unit) return `${it.name}${SEP}${it.amount}`
      return `${it.name}${SEP}${it.amount}${SEP}${it.unit}`
    })
    .join('\n')
}

export function formatCustomItemLabel(item: CustomItem): string {
  if (!item.amount) return item.name
  const unit = item.unit?.trim()
  return unit ? `${item.amount} ${unit} ${item.name}` : `${item.amount} ${item.name}`
}

export function formatCustomNameLabel(customName: string): string {
  return parseCustomItems(customName).map(formatCustomItemLabel).join('\n')
}
