import { formatCount } from '@/lib/observability'

describe('observability formatters', () => {
  it('formats counts with separators', () => {
    expect(formatCount(0)).toBe('0')
    expect(formatCount(1234567)).toBe((1234567).toLocaleString())
    expect(formatCount(42n)).toBe('42')
  })
})
