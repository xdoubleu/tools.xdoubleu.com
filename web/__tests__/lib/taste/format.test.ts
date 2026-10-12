import { DOMAINS, TRACKS, domainLabel, hostLabel, monthRows, signalBadge } from '@/lib/taste/format'

describe('DOMAINS', () => {
  it('lists every domain in display order with its series colour', () => {
    expect(DOMAINS.map((d) => d.id)).toEqual([
      'books',
      'feeds',
      'games',
      'movies',
      'series',
      'podcasts'
    ])
    expect(DOMAINS.map((d) => d.label)).toEqual([
      'Books',
      'Feeds',
      'Games',
      'Movies',
      'Series',
      'Podcasts'
    ])
    expect(DOMAINS.map((d) => d.color)).toEqual(
      [1, 2, 3, 4, 5, 6].map((n) => `rgb(var(--series-${n}-rgb))`)
    )
  })
})

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

describe('TRACKS', () => {
  it('lists more before broaden', () => {
    expect(TRACKS).toEqual([
      { value: 'more', label: 'More of what you like' },
      { value: 'broaden', label: 'Broaden your horizon' }
    ])
  })
})

describe('hostLabel', () => {
  it('shows the host without www., or the input when it is no URL', () => {
    expect(hostLabel('https://www.reddit.com/r/printSF')).toBe('reddit.com')
    expect(hostLabel('https://blog.example/post')).toBe('blog.example')
    expect(hostLabel('not a url')).toBe('not a url')
  })
})
