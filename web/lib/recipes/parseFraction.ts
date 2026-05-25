const UNICODE_FRACTIONS: Record<string, number> = {
  '½': 0.5,
  '¼': 0.25,
  '¾': 0.75,
  '⅓': 1 / 3,
  '⅔': 2 / 3,
  '⅛': 0.125,
  '⅜': 0.375,
  '⅝': 0.625,
  '⅞': 0.875
}

export function parseFraction(input: string): number {
  const s = input.trim()
  if (!s) return 0

  if (UNICODE_FRACTIONS[s] !== undefined) return UNICODE_FRACTIONS[s]

  for (const [sym, val] of Object.entries(UNICODE_FRACTIONS)) {
    if (s.endsWith(sym)) {
      const whole = parseFloat(s.slice(0, -sym.length))
      if (!isNaN(whole)) return whole + val
    }
  }

  const mixed = s.match(/^(\d+)\s+(\d+)\/(\d+)$/)
  if (mixed) return parseInt(mixed[1]) + parseInt(mixed[2]) / parseInt(mixed[3])

  const frac = s.match(/^(\d+)\/(\d+)$/)
  if (frac) return parseInt(frac[1]) / parseInt(frac[2])

  return parseFloat(s) || 0
}
