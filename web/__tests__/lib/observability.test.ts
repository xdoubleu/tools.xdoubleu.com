import { formatCount, formatDuration } from '@/lib/observability'

describe('observability formatters', () => {
  it('formats counts with separators', () => {
    expect(formatCount(0)).toBe('0')
    expect(formatCount(1234567)).toBe('1,234,567')
    expect(formatCount(42n)).toBe('42')
  })

  it('formats durations by magnitude', () => {
    expect(formatDuration(500)).toBe('500 ms')
    expect(formatDuration(1500)).toBe('1.5 s')
    expect(formatDuration(90_000)).toBe('1.5 min')
    expect(formatDuration(1500n)).toBe('1.5 s')
  })
})
