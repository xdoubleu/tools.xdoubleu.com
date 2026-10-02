// Mirrors the API's ToFraction (api/apps/recipes/fractions.go) for
// optimistic offline rendering of scaled ingredients.
const COMMON_FRACTIONS: [number, string][] = [
  [0, ''],
  [1 / 8, '⅛'],
  [1 / 4, '¼'],
  [1 / 3, '⅓'],
  [3 / 8, '⅜'],
  [1 / 2, '½'],
  [5 / 8, '⅝'],
  [2 / 3, '⅔'],
  [3 / 4, '¾'],
  [7 / 8, '⅞'],
  [1, '']
]

/** Formats an amount as a Unicode cooking fraction, e.g. 1.5 → "1½". */
export function toFraction(value: number): string {
  if (!(value > 0)) return '0'
  let whole = Math.floor(value)
  const frac = value - whole

  // `<=` keeps the later entry on a tie, like the API.
  let bestIdx = 0
  let bestDiff = Infinity
  COMMON_FRACTIONS.forEach(([v], i) => {
    const diff = Math.abs(frac - v)
    if (diff <= bestDiff) {
      bestDiff = diff
      bestIdx = i
    }
  })
  let symbol = COMMON_FRACTIONS[bestIdx][1]
  if (bestIdx === COMMON_FRACTIONS.length - 1) {
    whole++
    symbol = ''
  }

  if (whole === 0) return symbol || '0'
  return `${whole}${symbol}`
}
