import { formatBytes, formatCount, formatDuration, successRate } from '@/lib/observability'

describe('observability formatters', () => {
  it('formats bytes across units', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB')
    expect(formatBytes(3n * 1024n * 1024n * 1024n)).toBe('3.0 GB')
  })

  it('formats counts with separators', () => {
    expect(formatCount(0)).toBe('0')
    expect(formatCount(1234567)).toBe((1234567).toLocaleString())
    expect(formatCount(42n)).toBe('42')
  })

  it('formats durations', () => {
    expect(formatDuration(250)).toBe('250 ms')
    expect(formatDuration(1500)).toBe('1.5 s')
    expect(formatDuration(90000)).toBe('1.5 min')
    expect(formatDuration(500n)).toBe('500 ms')
  })

  it('computes success rate', () => {
    expect(successRate(0, 0)).toBe(100)
    expect(successRate(10, 0)).toBe(100)
    expect(successRate(10, 5)).toBe(50)
    expect(successRate(4n, 1n)).toBe(75)
  })
})
