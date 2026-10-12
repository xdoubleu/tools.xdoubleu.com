import { DOMAINS, domainLabel, monthRows, signalBadge } from '@/lib/taste/format'

describe('domainLabel', () => {
  it('labels known domains and passes unknown ones through', () => {
    expect(domainLabel('series')).toBe('Series')
    expect(domainLabel('music')).toBe('music')
  })
})

describe('signalBadge', () => {
  it.each([
    [2, 'Loved', 'success'],
    [1, 'Liked', 'success'],
    [0, 'Neutral', 'secondary'],
    [-1, 'Disliked', 'danger'],
    [-2, 'Hated', 'danger']
  ])('maps %d to %s', (signal, label, variant) => {
    expect(signalBadge(signal)).toEqual({ label, variant })
  })
})

describe('monthRows', () => {
  it('fills every domain with zero and labels the month', () => {
    const rows = monthRows([
      { month: '2026-01', counts: [{ domain: 'books', count: 3 }] },
      { month: '2026-02', counts: [] }
    ])
    expect(rows).toHaveLength(2)
    expect(rows[0].month).toBe('Jan 26')
    expect(rows[0].Books).toBe(3)
    expect(rows[0].Games).toBe(0)
    expect(Object.keys(rows[1])).toHaveLength(DOMAINS.length + 1)
  })
})
