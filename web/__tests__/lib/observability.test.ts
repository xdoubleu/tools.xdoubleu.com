import {
  formatCount,
  formatDuration,
  isAutomatedActionStale,
  AUTOMATED_ACTION_STALE_MINUTES
} from '@/lib/observability'

describe('observability formatters', () => {
  it('formats counts with separators', () => {
    expect(formatCount(0)).toBe('0')
    expect(formatCount(1234567)).toBe((1234567).toLocaleString())
    expect(formatCount(42n)).toBe('42')
  })

  it('formats durations by magnitude', () => {
    expect(formatDuration(500)).toBe('500 ms')
    expect(formatDuration(1500)).toBe('1.5 s')
    expect(formatDuration(90_000)).toBe('1.5 min')
    expect(formatDuration(1500n)).toBe('1.5 s')
  })
})

describe('isAutomatedActionStale', () => {
  beforeEach(() => {
    jest.useFakeTimers().setSystemTime(new Date('2026-01-01T12:00:00Z'))
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('is not stale once finished, regardless of how long it ran', () => {
    expect(isAutomatedActionStale('2026-01-01T00:00:00Z', '2026-01-01T13:00:00Z')).toBe(false)
  })

  it('is not stale while still open under the threshold', () => {
    const firedAt = new Date(
      Date.now() - (AUTOMATED_ACTION_STALE_MINUTES - 1) * 60_000
    ).toISOString()
    expect(isAutomatedActionStale(firedAt, '')).toBe(false)
  })

  it('is stale once still open past the threshold', () => {
    const firedAt = new Date(
      Date.now() - (AUTOMATED_ACTION_STALE_MINUTES + 1) * 60_000
    ).toISOString()
    expect(isAutomatedActionStale(firedAt, '')).toBe(true)
  })

  it('is not stale for an unparseable fired_at', () => {
    expect(isAutomatedActionStale('not-a-date', '')).toBe(false)
  })
})
