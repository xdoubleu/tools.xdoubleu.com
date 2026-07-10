import { formatDate, formatDateTime, oneYearAgo, today } from '@/lib/dates'

describe('backlog date helpers', () => {
  it('today returns an ISO date string for now', () => {
    jest.useFakeTimers().setSystemTime(new Date('2026-06-08T12:00:00Z'))
    expect(today()).toBe('2026-06-08')
    jest.useRealTimers()
  })

  it('oneYearAgo returns the same day one year earlier', () => {
    jest.useFakeTimers().setSystemTime(new Date('2026-06-08T12:00:00Z'))
    expect(oneYearAgo()).toBe('2025-06-08')
    jest.useRealTimers()
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
