import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('recharts', () => ({
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

jest.mock('@/hooks/useFeeds', () => ({
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
    expect(screen.getByText('82%')).toBeInTheDocument()

    expect(screen.getByText('Untitled feed')).toBeInTheDocument()
    expect(screen.getByText('not enough data yet')).toBeInTheDocument()
  })
})
