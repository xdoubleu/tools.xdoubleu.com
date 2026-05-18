/**
 * Formats an ISO date string (YYYY-MM-DD) to a short month+day string,
 * e.g. "2024-01-15" → "Jan 15".
 * Returns null for null input.
 */
export function formatDueDate(isoDate: string | null): string | null {
  if (isoDate === null) return null
  const date = new Date(`${isoDate}T00:00:00`)
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

/**
 * Returns today's date as a YYYY-MM-DD string in local time.
 */
function todayStr(): string {
  const now = new Date()
  const y = now.getFullYear()
  const m = String(now.getMonth() + 1).padStart(2, '0')
  const d = String(now.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/**
 * Returns tomorrow's date as a YYYY-MM-DD string in local time.
 */
function tomorrowStr(): string {
  const now = new Date()
  now.setDate(now.getDate() + 1)
  const y = now.getFullYear()
  const m = String(now.getMonth() + 1).padStart(2, '0')
  const d = String(now.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/**
 * Returns true if the given ISO date is strictly before today (overdue).
 */
export function isOverdue(isoDate: string | null): boolean {
  if (isoDate === null) return false
  return isoDate < todayStr()
}

/**
 * Returns true if the given ISO date is today.
 */
export function isDueToday(isoDate: string | null): boolean {
  if (isoDate === null) return false
  return isoDate === todayStr()
}

/**
 * Returns a human-readable relative date label:
 * - "Today" if the date is today
 * - "Tomorrow" if the date is tomorrow
 * - Short formatted date otherwise (e.g. "Jan 15")
 */
export function formatRelativeDate(isoDate: string): string {
  if (isoDate === todayStr()) return 'Today'
  if (isoDate === tomorrowStr()) return 'Tomorrow'
  return formatDueDate(isoDate) ?? isoDate
}
