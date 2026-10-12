import React from 'react'
import { render, screen, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockBarChart = jest.fn()

jest.mock('recharts', () => ({
  BarChart: (props: { data: unknown; children: React.ReactNode }) => {
    mockBarChart(props.data)
    return <div>{props.children}</div>
  },
  Bar: ({ dataKey, stackId, fill }: { dataKey: string; stackId: string; fill: string }) => (
    <span data-testid="bar">{`${dataKey}:${stackId}:${fill}`}</span>
  ),
  XAxis: () => null,
  YAxis: ({ allowDecimals }: { allowDecimals: boolean }) => (
    <span data-testid="y-axis">{String(allowDecimals)}</span>
  ),
  CartesianGrid: () => null,
  Tooltip: () => null,
  Legend: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>
}))

jest.mock('@/hooks/useTaste', () => ({ useTasteStats: jest.fn() }))

import TasteStats from '@/components/taste/TasteStats'
import { useTasteStats } from '@/hooks/useTaste'
import {
  DomainCountSchema,
  DomainStatsSchema,
  GetStatsResponseSchema,
  HitRateSchema,
  MonthCountsSchema,
  RankedNameSchema
} from '@/lib/gen/taste/v1/taste_pb'

function mockStats(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useTasteStats).mockReturnValue({ isLoading: false, ...value })
}

const domains = ['books', 'feeds', 'games', 'movies', 'series', 'podcasts'].map((domain) =>
  create(DomainStatsSchema, { domain })
)

const full = create(GetStatsResponseSchema, {
  domains: [
    create(DomainStatsSchema, {
      domain: 'books',
      consumed: 4,
      positive: 3,
      negative: 1,
      lastAt: '2026-10-02T10:00:00Z'
    }),
    ...domains.slice(1)
  ],
  months: [
    create(MonthCountsSchema, {
      month: '2026-10',
      counts: [create(DomainCountSchema, { domain: 'books', count: 4 })]
    })
  ],
  topTags: [
    create(RankedNameSchema, { name: 'sci-fi', positive: 3, domains: ['books', 'movies'] })
  ],
  topCreators: []
})

beforeEach(() => jest.clearAllMocks())

describe('TasteStats', () => {
  it('shows loading and error states without data', () => {
    mockStats({ isLoading: true })
    const { rerender } = render(<TasteStats />)
    expect(screen.getByText(/Loading stats/)).toBeInTheDocument()

    mockStats({ error: new Error('nope') })
    rerender(<TasteStats />)
    expect(screen.getByRole('alert')).toBeInTheDocument()

    mockStats({})
    const { container } = render(<TasteStats />)
    expect(container).toBeEmptyDOMElement()
  })

  it('asks to consume something first', () => {
    mockStats({ data: create(GetStatsResponseSchema, { domains }) })
    render(<TasteStats />)
    expect(screen.getByText(/Nothing consumed yet/)).toBeInTheDocument()
  })

  it('keeps loaded stats after a failed refresh', () => {
    mockStats({ data: full, error: new Error('nope'), isLoading: true })
    render(<TasteStats />)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByText('3 liked · 1 disliked · last 02/10/2026')).toBeInTheDocument()
  })

  it('shows tiles, the stacked chart and the ranked lists', () => {
    mockStats({ data: full })
    render(<TasteStats />)

    expect(screen.getByText('3 liked · 1 disliked · last 02/10/2026')).toBeInTheDocument()
    expect(screen.getAllByText('0 liked · 0 disliked')).toHaveLength(5)
    expect(screen.getAllByTestId('bar').map((b) => b.textContent)).toContain(
      'Books:events:rgb(var(--series-1-rgb))'
    )
    expect(screen.getByTestId('y-axis')).toHaveTextContent('false')
    expect(mockBarChart).toHaveBeenCalledWith([
      expect.objectContaining({ month: 'Oct 26', Books: 4 })
    ])

    const tags = screen.getByText('Top tags').closest('div')!.parentElement!
    expect(within(tags).getByText('sci-fi')).toBeInTheDocument()
    expect(within(tags).getByText('Movies')).toBeInTheDocument()
    expect(within(tags).getByText('3')).toBeInTheDocument()
    expect(screen.getByText('No liked creators yet.')).toBeInTheDocument()
    expect(screen.queryByText('Recommendation hit rate')).not.toBeInTheDocument()
  })

  it('shows the hit rate per domain and track', () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {})
    mockStats({
      data: create(GetStatsResponseSchema, {
        ...full,
        hitRates: [
          create(HitRateSchema, { domain: 'books', track: 'more', hits: 2, misses: 1 }),
          create(HitRateSchema, { domain: 'series', track: 'broaden', hits: 0, misses: 2 })
        ]
      })
    })
    render(<TasteStats />)
    const list = screen.getByRole('list', { name: 'Hit rate per domain and track' })
    const [books, series] = within(list).getAllByRole('listitem')
    expect(books).toHaveTextContent('Books · More of what you like67% (2 of 3)')
    expect(series).toHaveTextContent('Series · Broaden your horizon0% (0 of 2)')
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
  })
})
