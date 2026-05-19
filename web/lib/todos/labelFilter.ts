/**
 * Filters label presets by a query string (case-insensitive substring match).
 */
export function filterLabels(presets: string[], query: string): string[] {
  const q = query.trim().toLowerCase()
  if (q === '') return presets
  return presets.filter((preset) => preset.toLowerCase().includes(q))
}

/**
 * Returns true if the label contains the query string (case-insensitive).
 */
export function matchesLabel(label: string, query: string): boolean {
  return label.toLowerCase().includes(query.trim().toLowerCase())
}

/**
 * Splits a comma-separated label string into trimmed, non-empty label values.
 * e.g. "label1, label2" → ["label1", "label2"]
 */
export function normalizeLabels(raw: string): string[] {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
}
