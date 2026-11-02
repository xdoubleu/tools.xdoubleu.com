const HOUR_MS = 3_600_000
const DAY_MS = 24 * HOUR_MS

// expiresLabel says how long an unread item has left ("Expires in 5h"), or
// null when it never expires or has more than two days.
export function expiresLabel(expiresAt: string, now: number = Date.now()): string | null {
  if (!expiresAt) return null
  const left = new Date(expiresAt).getTime() - now
  if (Number.isNaN(left) || left > 2 * DAY_MS) return null
  if (left <= HOUR_MS) return 'Expires within the hour'
  if (left < DAY_MS) return `Expires in ${Math.floor(left / HOUR_MS)}h`
  return 'Expires tomorrow'
}

// Lifetime choices in days; 0 never expires.
export const EXPIRY_OPTIONS = [1, 3, 7, 14, 30, 0] as const

export function expiryOptionLabel(days: number): string {
  if (days === 0) return 'Never'
  return days === 1 ? '1 day' : `${days} days`
}
