import {
  axisDate,
  formatDate,
  formatDateRange,
  formatDateTime,
  formatRelative,
  formatTime,
  monthLabel,
  oneYearAgo,
  today
} from '@/lib/dates'

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

const NOW = new Date(2026, 10, 11, 12, 0)

describe('formatDate', () => {
  it('drops the year in the current year', () => {
    expect(formatDate('2026-01-15', NOW)).toBe('15 Jan')
  })

  it('keeps the year in other years, reading date-only strings as local days', () => {
    expect(formatDate('2019-01-01', NOW)).toBe('1 Jan 2019')
  })

  it('formats RFC3339 timestamps and Date objects', () => {
    expect(formatDate(new Date(2025, 5, 3), NOW)).toBe('3 Jun 2025')
    expect(formatDate(new Date(2026, 5, 3, 10, 30).toISOString(), NOW)).toBe('3 Jun')
  })

  it('returns an empty string for empty or unparseable input', () => {
    expect(formatDate('')).toBe('')
    expect(formatDate('not a date')).toBe('')
    expect(formatDate('x2026-01-15')).toBe('')
  })
})

describe('formatTime and formatDateTime', () => {
  it('prints hours and minutes without seconds', () => {
    expect(formatTime(new Date(2026, 0, 5, 9, 7, 42))).toBe('09:07')
    expect(formatDateTime(new Date(2025, 0, 15, 10, 30), NOW)).toBe('15 Jan 2025, 10:30')
    expect(formatDateTime(new Date(2026, 0, 15, 10, 30), NOW)).toBe('15 Jan, 10:30')
  })

  it('returns an empty string for empty or unparseable input', () => {
    expect(formatTime('')).toBe('')
    expect(formatDateTime('')).toBe('')
    expect(formatDateTime('not a date')).toBe('')
  })
})

describe('formatDateRange', () => {
  it.each([
    [new Date(2026, 10, 11), new Date(2026, 10, 17), '11–17 Nov'],
    [new Date(2026, 9, 28), new Date(2026, 10, 3), '28 Oct – 3 Nov'],
    [new Date(2025, 11, 29), new Date(2026, 0, 4), '29 Dec 2025 – 4 Jan'],
    [new Date(2026, 10, 11), new Date(2026, 10, 11), '11 Nov'],
    [new Date(2025, 10, 11), new Date(2025, 10, 17), '11–17 Nov 2025']
  ])('%s to %s is %s', (a, b, expected) => {
    expect(formatDateRange(a, b, NOW)).toBe(expected)
  })

  it('is empty when neither end parses', () => {
    expect(formatDateRange('', 'nope', NOW)).toBe('')
  })

  it('falls back to the one valid end', () => {
    expect(formatDateRange('', '2026-11-17', NOW)).toBe('17 Nov')
  })
})

describe('formatRelative', () => {
  const at = (ms: number) => new Date(NOW.getTime() + ms)
  it.each([
    [-10_000, 'just now'],
    [-60_000, '1 min ago'],
    [-5 * 60_000, '5 min ago'],
    [-3_600_000, '1 h ago'],
    [7 * 24 * 3_600_000, 'in 7 days'],
    [-7 * 24 * 3_600_000, '7 days ago'],
    [3 * 3_600_000, 'in 3 h'],
    [-24 * 3_600_000, 'yesterday'],
    [24 * 3_600_000, 'tomorrow'],
    [-3 * 24 * 3_600_000, '3 days ago'],
    [-30 * 24 * 3_600_000, '12 Oct']
  ])('%d ms is %s', (ms, expected) => {
    expect(formatRelative(at(ms), NOW)).toBe(expected)
  })

  it('returns an empty string for unparseable input', () => {
    expect(formatRelative('nope', NOW)).toBe('')
  })
})

describe('axisDate', () => {
  it('shortens dates and passes other labels through', () => {
    expect(axisDate('2019-01-01')).toBe('1 Jan 2019')
    expect(axisDate('Week 3')).toBe('Week 3')
    expect(axisDate('x2019-01-01')).toBe('x2019-01-01')
    expect(axisDate('2019-01-1')).toBe('2019-01-1')
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
