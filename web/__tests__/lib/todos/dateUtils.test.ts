import {
  formatDueDate,
  isOverdue,
  isDueToday,
  formatRelativeDate,
} from '@/lib/todos/dateUtils'

// Helper to build a YYYY-MM-DD string offset from today by `days` days.
function offsetDate(days: number): string {
  const d = new Date()
  d.setDate(d.getDate() + days)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

describe('formatDueDate', () => {
  it('returns null for null input', () => {
    expect(formatDueDate(null)).toBeNull()
  })

  it('formats a date into short month+day string', () => {
    expect(formatDueDate('2024-01-15')).toBe('Jan 15')
  })

  it('formats a December date correctly', () => {
    expect(formatDueDate('2024-12-31')).toBe('Dec 31')
  })

  it('formats a single-digit day without zero-padding in output', () => {
    // "Jan 5" not "Jan 05"
    expect(formatDueDate('2024-01-05')).toBe('Jan 5')
  })
})

describe('isOverdue', () => {
  it('returns false for null input', () => {
    expect(isOverdue(null)).toBe(false)
  })

  it('returns true for a date in the past', () => {
    expect(isOverdue('2000-01-01')).toBe(true)
  })

  it('returns false for today', () => {
    expect(isOverdue(offsetDate(0))).toBe(false)
  })

  it('returns false for a future date', () => {
    expect(isOverdue(offsetDate(1))).toBe(false)
  })
})

describe('isDueToday', () => {
  it('returns false for null input', () => {
    expect(isDueToday(null)).toBe(false)
  })

  it('returns true for today\'s date', () => {
    expect(isDueToday(offsetDate(0))).toBe(true)
  })

  it('returns false for yesterday', () => {
    expect(isDueToday(offsetDate(-1))).toBe(false)
  })

  it('returns false for tomorrow', () => {
    expect(isDueToday(offsetDate(1))).toBe(false)
  })
})

describe('formatRelativeDate', () => {
  it('returns "Today" for today\'s date', () => {
    expect(formatRelativeDate(offsetDate(0))).toBe('Today')
  })

  it('returns "Tomorrow" for tomorrow\'s date', () => {
    expect(formatRelativeDate(offsetDate(1))).toBe('Tomorrow')
  })

  it('returns formatted date string for a past date', () => {
    // January 15 2024 is not today or tomorrow
    const result = formatRelativeDate('2024-01-15')
    expect(result).toBe('Jan 15')
  })

  it('returns formatted date string for a far future date', () => {
    // A date far in the future that is neither today nor tomorrow
    const result = formatRelativeDate('2099-06-20')
    expect(result).toBe('Jun 20')
  })
})
