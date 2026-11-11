export function oneYearAgo(): string {
  const d = new Date()
  d.setFullYear(d.getFullYear() - 1)
  return d.toISOString().slice(0, 10)
}

export function today(): string {
  return new Date().toISOString().slice(0, 10)
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

interface Day {
  year: number
  month: number
  day: number
}

// Stryker disable Regex,ConditionalExpression,StringLiteral,EqualityOperator: only a negative UTC offset tells the local-day read from `new Date(value)`, and tests can't set the zone.
/** Date-only strings are read as local days: `new Date('yyyy-MM-dd')` is UTC and shifts one in negative offsets. */
function parseDate(value: string | Date): Date | null {
  if (!value) return null
  if (typeof value === 'string') {
    const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
    if (m) return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
  }
  const date = typeof value === 'string' ? new Date(value) : value
  return Number.isNaN(date.getTime()) ? null : date
}

// Stryker restore Regex,ConditionalExpression,StringLiteral,EqualityOperator

function day(date: Date): Day {
  return { year: date.getFullYear(), month: date.getMonth(), day: date.getDate() }
}

function dayLabel(d: Day, now: Date): string {
  const base = `${d.day} ${MONTHS[d.month]}`
  return d.year === now.getFullYear() ? base : `${base} ${d.year}`
}

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

/** "11 Nov 2026"; the year is dropped in the current year ("11 Nov"). '' for empty or unparseable input. */
export function formatDate(value: string | Date, now: Date = new Date()): string {
  const date = parseDate(value)
  return date ? dayLabel(day(date), now) : ''
}

/** "14:30" in the viewer's zone. '' for empty or unparseable input. */
export function formatTime(value: string | Date): string {
  const date = parseDate(value)
  return date ? `${pad(date.getHours())}:${pad(date.getMinutes())}` : ''
}

/** "11 Nov 2026, 14:30" (no seconds). '' for empty or unparseable input. */
export function formatDateTime(value: string | Date, now: Date = new Date()): string {
  const date = parseDate(value)
  return date ? `${dayLabel(day(date), now)}, ${formatTime(date)}` : ''
}

/** "11–17 Nov 2026" within a month, "28 Oct – 3 Nov 2026" across months; each end in full across years. */
export function formatDateRange(
  start: string | Date,
  end: string | Date,
  now: Date = new Date()
): string {
  const a = parseDate(start)
  const b = parseDate(end)
  if (!a || !b) {
    const one = a ?? b
    return one ? formatDate(one, now) : ''
  }
  const da = day(a)
  const db = day(b)
  if (da.year !== db.year) return `${dayLabel(da, now)} – ${dayLabel(db, now)}`
  if (da.month !== db.month) {
    return `${da.day} ${MONTHS[da.month]} – ${dayLabel(db, now)}`
  }
  if (da.day === db.day) return dayLabel(da, now)
  return `${da.day}–${dayLabel(db, now)}`
}

const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/** "just now", "5 min ago", "in 3 h", "yesterday", "3 days ago"; older than a week falls back to the short date. */
export function formatRelative(value: string | Date, now: Date = new Date()): string {
  const date = parseDate(value)
  if (!date) return ''
  const diff = date.getTime() - now.getTime()
  const abs = Math.abs(diff)
  // Stryker disable next-line EqualityOperator: diff is never 0 past the "just now" check.
  const wrap = (n: string) => (diff < 0 ? `${n} ago` : `in ${n}`)
  if (abs < MINUTE) return 'just now'
  if (abs < HOUR) return wrap(`${Math.round(abs / MINUTE)} min`)
  if (abs < DAY) return wrap(`${Math.round(abs / HOUR)} h`)
  const days = Math.round(abs / DAY)
  // Stryker disable next-line EqualityOperator: diff is never 0 past the "just now" check.
  if (days === 1) return diff < 0 ? 'yesterday' : 'tomorrow'
  if (days <= 7) return wrap(`${days} days`)
  return formatDate(date, now)
}

/** A chart axis label: dates become the short date, anything else (a month name, "Week 3") passes through. */
export function axisDate(label: string): string {
  return /^\d{4}-\d{2}-\d{2}/.test(label) ? formatDate(label) : label
}

/** "2026-01" → "Jan 26", in fixed English. */
export function monthLabel(month: string): string {
  const [year, m] = month.split('-')
  return `${MONTHS[Number(m) - 1]} ${year.slice(2)}`
}
