// Custom (recipe-less) meal entries store hand-typed items as a newline-separated
// list in `customName`. Each line is `name`, `name<TAB>amount`, or
// `name<TAB>amount<TAB>unit`. The tab separator is used because it cannot be
// typed into a single-line text input.

export interface CustomItem {
  name: string
  amount: string
  unit?: string
  // UI-only: the category chosen in the entry form. It is written to the
  // name->category catalog on save and is never encoded into `customName`.
  categoryId?: string
}

const SEP = '\t'

// parseCustomItems decodes a stored `customName` into structured items, dropping
// blank lines.
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

// encodeCustomItems joins structured items back into the stored newline format,
// dropping items with a blank name.
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

// formatCustomItemLabel renders an item for display, e.g. "2 kg apples" or "apples".
export function formatCustomItemLabel(item: CustomItem): string {
  if (!item.amount) return item.name
  const unit = item.unit?.trim()
  return unit ? `${item.amount} ${unit} ${item.name}` : `${item.amount} ${item.name}`
}

// formatCustomNameLabel renders a whole stored `customName` for display, one
// item per line, stripping the tab separator.
export function formatCustomNameLabel(customName: string): string {
  return parseCustomItems(customName).map(formatCustomItemLabel).join('\n')
}
