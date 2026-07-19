import { create } from '@bufbuild/protobuf'
import { UsageDaySchema } from '@/lib/gen/observability/v1/observability_pb'
import { aggregateUsage, OTHER_LABEL, MAX_SERIES } from '@/components/monitoring/usageData'

function day(d: string, app: string, endpoint: string, count: number) {
  return create(UsageDaySchema, { day: d, app, endpoint, count: BigInt(count) })
}

describe('aggregateUsage', () => {
  it('sums per-day per-app totals and sorts days', () => {
    const { rows, apps } = aggregateUsage([
      day('2026-01-02', 'books', 'a', 2),
      day('2026-01-01', 'books', 'a', 1),
      day('2026-01-01', 'books', 'b', 3),
      day('2026-01-01', 'games', 'x', 5)
    ])

    expect(rows.map((r) => r.day)).toEqual(['2026-01-01', '2026-01-02'])
    expect(rows[0]['books']).toBe(4)
    expect(rows[0]['games']).toBe(5)
    expect(rows[1]['books']).toBe(2)
    expect(apps).toEqual(expect.arrayContaining(['books', 'games']))
  })

  it('buckets apps beyond MAX_SERIES into Other', () => {
    const entries = []
    // Create MAX_SERIES + 2 apps, each with a distinct volume.
    for (let i = 0; i < MAX_SERIES + 2; i++) {
      entries.push(day('2026-01-01', `app${i}`, 'root', (MAX_SERIES + 2 - i) * 10))
    }
    const { apps, rows } = aggregateUsage(entries)

    expect(apps).toContain(OTHER_LABEL)
    // Only MAX_SERIES named apps plus Other.
    expect(apps.length).toBe(MAX_SERIES + 1)
    // The two smallest apps collapse into Other.
    expect(Number(rows[0][OTHER_LABEL])).toBe(10 + 20)
  })

  it('returns endpoints sorted by count desc', () => {
    const { endpoints } = aggregateUsage([
      day('2026-01-01', 'books', 'small', 1),
      day('2026-01-01', 'books', 'big', 100),
      day('2026-01-02', 'books', 'big', 50)
    ])

    expect(endpoints[0]).toEqual({ app: 'books', endpoint: 'big', count: 150 })
    expect(endpoints[1]).toEqual({ app: 'books', endpoint: 'small', count: 1 })
  })

  it('handles empty input', () => {
    const { rows, apps, endpoints } = aggregateUsage([])
    expect(rows).toEqual([])
    expect(apps).toEqual([])
    expect(endpoints).toEqual([])
  })
})
