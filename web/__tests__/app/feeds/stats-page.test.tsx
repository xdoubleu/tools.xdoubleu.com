import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/feeds/FeedStatsClient', () => () => <div data-testid="feed-stats-client" />)

const mockServerClient = {
  getFeedStats: jest.fn()
}
jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => mockServerClient)
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

const fallbackProps: Record<string, unknown>[] = []
jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({
    children,
    fallback
  }: {
    children: React.ReactNode
    fallback: Record<string, unknown>
  }) => {
    fallbackProps.push(fallback)
    return <>{children}</>
  }
}))

import FeedStatsPage from '@/app/feeds/stats/page'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'

describe('FeedStatsPage', () => {
  beforeEach(() => {
    fallbackProps.length = 0
  })

  it('renders the stats client', async () => {
    render(await FeedStatsPage())
    expect(screen.getByTestId('feed-stats-client')).toBeInTheDocument()
    expect(fallbackProps[0]).toEqual({})
  })

  it('prefetches the stats', async () => {
    const stats = { stats: [], itemsPerDay: [] }
    mockServerClient.getFeedStats.mockResolvedValueOnce(stats)
    jest.mocked(fetchOrNull).mockImplementationOnce((fetcher) => fetcher())
    render(await FeedStatsPage())
    expect(fallbackProps[0]).toEqual({ [swrKeys.feedStats]: stats })
  })

  it('shows a breadcrumb back to /feeds', async () => {
    render(await FeedStatsPage())
    expect(screen.getByRole('link', { name: 'Feeds' })).toHaveAttribute('href', '/feeds')
    expect(screen.getByText('Stats')).toHaveAttribute('aria-current', 'page')
  })
})
