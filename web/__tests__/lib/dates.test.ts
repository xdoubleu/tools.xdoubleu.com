import { oneYearAgo, today } from '@/lib/dates'

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
