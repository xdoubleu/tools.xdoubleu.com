// Custom (recipe-less) meal entries store hand-typed items as a newline-separated
// list in `customName`. Each line is either a bare `name` or `name<TAB>amount`,
// where the optional amount flows through to the shopping-list export. The tab
// separator is used because it cannot be typed into a single-line text input.

export interface CustomItem {
  name: string
  amount: string
  // UI-only: the category chosen in the entry form. It is written to the
  // name->category catalog on save and is never encoded into `customName`.
  categoryId?: string
}

const SEP = '\t'

// parseCustomItems decodes a stored `customName` into structured items, dropping
// blank lines. A line without a tab has no amount.
export function parseCustomItems(customName: string): CustomItem[] {
  return customName
    .split('\n')
    .filter(Boolean)
    .map((line) => {
      const idx = line.indexOf(SEP)
      if (idx === -1) return { name: line, amount: '' }
      return { name: line.slice(0, idx), amount: line.slice(idx + 1) }
    })
}

// encodeCustomItems joins structured items back into the stored newline format,
// dropping items with a blank name and appending the amount only when set.
export function encodeCustomItems(items: CustomItem[]): string {
  return items
    .map((it) => ({ name: it.name.trim(), amount: it.amount.trim() }))
    .filter((it) => it.name)
    .map((it) => (it.amount ? `${it.name}${SEP}${it.amount}` : it.name))
    .join('\n')
}

// formatCustomItemLabel renders an item for display, e.g. "2 apples" or "apples".
export function formatCustomItemLabel(item: CustomItem): string {
  return item.amount ? `${item.amount} ${item.name}` : item.name
}

// formatCustomNameLabel renders a whole stored `customName` for display, one
// item per line, stripping the tab separator.
export function formatCustomNameLabel(customName: string): string {
  return parseCustomItems(customName).map(formatCustomItemLabel).join('\n')
}
