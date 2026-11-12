import { seriesColor } from '@/lib/chartTheme'

describe('seriesColor', () => {
  it('reads the nth series token', () => {
    expect(seriesColor(1)).toBe('rgb(var(--series-1-rgb))')
    expect(seriesColor(6)).toBe('rgb(var(--series-6-rgb))')
  })
})
