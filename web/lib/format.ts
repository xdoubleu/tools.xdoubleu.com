/** A number with at most `decimals` decimals (default 1) and grouped thousands: 933.333 → "933.3", 12345 → "12,345". '' if blank or not finite. */
export function formatNumber(value: number | string | bigint, decimals = 1): string {
  if (typeof value === 'string' && value.trim() === '') return ''
  const n = Number(value)
  if (!Number.isFinite(n)) return ''
  const scale = 10 ** decimals
  const rounded = Math.round(Math.abs(n) * scale) / scale
  const [integer, decimal] = String(rounded).split('.')
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  // Stryker disable next-line EqualityOperator: n is never 0 with a non-zero rounded value.
  const sign = n < 0 && rounded !== 0 ? '-' : ''
  return decimal ? `${sign}${grouped}.${decimal}` : `${sign}${grouped}`
}

/** A percentage given as 0–100, at most one decimal: "66.50" → "66.5%". '' for NaN. */
export function formatPercent(value: number | string): string {
  const text = formatNumber(value)
  return text && `${text}%`
}

/** "1 comment", "2 comments"; pass `plural` for irregular words ("1 person", "2 people"). */
export function pluralize(count: number, singular: string, plural = `${singular}s`): string {
  return `${formatNumber(count)} ${count === 1 ? singular : plural}`
}

/**
 * "500 g coffee beans", "0.25 kg flour" (up to two decimals), "6 bananas"; a zero or missing amount leaves just the name
 * so a free-text line doesn't print "0 — dinner at sam's".
 */
export function formatQuantity(
  amount: number | string | undefined,
  unit: string,
  name: string
): string {
  const n = parseFloat(String(amount))
  if (!Number.isFinite(n) || n === 0) return name
  return [formatNumber(n, 2), unit.trim(), name].filter(Boolean).join(' ')
}
