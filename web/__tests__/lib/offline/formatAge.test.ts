import { formatAge } from '@/lib/offline/formatAge'

describe('formatAge', () => {
  it.each([
    [0, 'just now'],
    [59_999, 'just now'],
    [60_000, '1 min ago'],
    [59 * 60_000, '59 min ago'],
    [60 * 60_000, '1 h ago'],
    [23 * 3_600_000, '23 h ago'],
    [24 * 3_600_000, '1 d ago'],
    [50 * 3_600_000, '2 d ago']
  ])('%i ms → %s', (ms, label) => {
    expect(formatAge(ms)).toBe(label)
  })
})
