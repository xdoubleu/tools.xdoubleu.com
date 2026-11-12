import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('recharts', () => ({
  BarChart: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Bar: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>
}))

const feedStatsData: {
  data?: { stats: unknown[]; itemsPerDay: unknown[] }
  error?: Error
  isLoading: boolean
} = { data: undefined, error: undefined, isLoading: false }

vi.mock('@/hooks/useFeeds', () => ({
  useFeedStats: () => feedStatsData
}))

import FeedStatsClient from '@/components/feeds/FeedStatsClient'

describe('FeedStatsClient', () => {
  beforeEach(() => {
    feedStatsData.data = undefined
    feedStatsData.error = undefined
    feedStatsData.isLoading = false
  })

  it('shows a loading state', () => {
    feedStatsData.isLoading = true
    render(<FeedStatsClient />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    feedStatsData.error = new Error('boom')
    render(<FeedStatsClient />)
    expect(screen.getByText('Failed to load feed stats.')).toBeInTheDocument()
  })

  it('shows an empty state when there are no feeds', () => {
    feedStatsData.data = { stats: [], itemsPerDay: [] }
    render(<FeedStatsClient />)
    expect(screen.getByText('No feed activity yet.')).toBeInTheDocument()
  })

  it('renders per-feed stats cards with formatted cadence and rates', () => {
    feedStatsData.data = {
      stats: [
        {
          feedId: 'feed-1',
          feedTitle: 'My Blog',
          itemCount: 12,
          avgIntervalHours: 36,
          readRate: 0.75,
          avgReadProgressPct: 82.4
        },
        {
          feedId: 'feed-2',
          feedTitle: '',
          itemCount: 1,
          avgIntervalHours: 0,
          readRate: 0,
          avgReadProgressPct: 0
        }
      ],
      itemsPerDay: [{ day: '2026-01-01T00:00:00Z', count: 3 }]
    }
    render(<FeedStatsClient />)

    expect(screen.getByText('My Blog')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('every ~1.5d')).toBeInTheDocument()
    expect(screen.getByText('75%')).toBeInTheDocument()
    expect(screen.getByText('82.4%')).toBeInTheDocument()

    expect(screen.getByText('Untitled feed')).toBeInTheDocument()
    expect(screen.getByText('not enough data yet')).toBeInTheDocument()
  })

  const feed = (feedId: string, itemCount: number) => ({
    feedId,
    feedTitle: feedId,
    itemCount,
    avgIntervalHours: 0,
    readRate: 0,
    avgReadProgressPct: 0
  })

  it('totals the feeds, their items and the last 90 days in tiles', () => {
    feedStatsData.data = {
      stats: [feed('a', 100), feed('b', 20), feed('c', 3)],
      itemsPerDay: [
        { day: '2026-01-01T00:00:00Z', count: 4 },
        { day: '2026-01-02T00:00:00Z', count: 5 }
      ]
    }
    render(<FeedStatsClient />)
    // Feed cards repeat the "Items" label; the tiles come first.
    const tile = (label: string) => screen.getAllByText(label)[0].parentElement
    expect(tile('Feeds')).toHaveTextContent(/^Feeds3$/)
    expect(tile('Items')).toHaveTextContent(/^Items123$/)
    expect(tile('Last 90 days')).toHaveTextContent(/^Last 90 days9$/)
    expect(screen.queryByText('No items ingested in the last 90 days.')).not.toBeInTheDocument()
  })

  it('says so when nothing was ingested in the last 90 days', () => {
    feedStatsData.data = { stats: [feed('a', 5)], itemsPerDay: [] }
    render(<FeedStatsClient />)
    expect(screen.getByText('No items ingested in the last 90 days.')).toBeInTheDocument()
  })
})
