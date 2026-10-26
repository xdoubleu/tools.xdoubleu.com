import { formatDate, formatDateTime, monthLabel, oneYearAgo, today } from '@/lib/dates'

describe('backlog date helpers', () => {
  it('today returns an ISO date string for now', () => {
    vi.useFakeTimers().setSystemTime(new Date('2026-06-08T12:00:00Z'))
    expect(today()).toBe('2026-06-08')
    vi.useRealTimers()
  })

  it('oneYearAgo returns the same day one year earlier', () => {
    vi.useFakeTimers().setSystemTime(new Date('2026-06-08T12:00:00Z'))
    expect(oneYearAgo()).toBe('2025-06-08')
    vi.useRealTimers()
  })
})

describe('formatDate', () => {
  it('reorders date-only strings without timezone shifts', () => {
    expect(formatDate('2026-01-15')).toBe('15/01/2026')
  })

  it('formats RFC3339 timestamps as dd/MM/yyyy', () => {
    expect(formatDate('2026-01-15T10:30:00Z')).toMatch(/^15\/01\/2026$/)
  })

  it('formats Date objects as dd/MM/yyyy', () => {
    expect(formatDate(new Date(2026, 0, 15))).toBe('15/01/2026')
  })

  it('returns an empty string for empty input', () => {
    expect(formatDate('')).toBe('')
  })

  it('returns an empty string for unparseable input', () => {
    expect(formatDate('not a date')).toBe('')
  })
})

describe('formatDateTime', () => {
  it('formats timestamps with a dd/MM/yyyy date part', () => {
    expect(formatDateTime(new Date(2026, 0, 15, 10, 30))).toMatch(/15\/01\/2026/)
  })

  it('returns an empty string for empty input', () => {
    expect(formatDateTime('')).toBe('')
  })

  it('returns an empty string for unparseable input', () => {
    expect(formatDateTime('not a date')).toBe('')
  })
})

describe('monthLabel', () => {
  it('names every month', () => {
    const months = Array.from({ length: 12 }, (_, i) =>
      monthLabel(`2026-${String(i + 1).padStart(2, '0')}`)
    )
    expect(months.join(' ')).toBe(
      'Jan 26 Feb 26 Mar 26 Apr 26 May 26 Jun 26 Jul 26 Aug 26 Sep 26 Oct 26 Nov 26 Dec 26'
    )
    expect(monthLabel('2025-12')).toBe('Dec 25')
  })
})
