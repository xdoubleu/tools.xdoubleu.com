import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/feeds/FeedStatsClient', () => () => <div data-testid="feed-stats-client" />)
jest.mock('@/components/feeds/FeedRuleSuggestionsCard', () => () => (
  <div data-testid="feed-rule-suggestions" />
))

const mockServerClient = {
  getFeedStats: jest.fn(),
  getFilterRuleSuggestions: jest.fn()
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

  it('renders the rule suggestions and the stats client', async () => {
    render(await FeedStatsPage())
    expect(screen.getByTestId('feed-rule-suggestions')).toBeInTheDocument()
    expect(screen.getByTestId('feed-stats-client')).toBeInTheDocument()
    expect(fallbackProps[0]).toEqual({})
  })

  it('prefetches the stats and the suggestions', async () => {
    const stats = { stats: [], itemsPerDay: [] }
    const suggestions = { suggestions: [] }
    mockServerClient.getFeedStats.mockResolvedValueOnce(stats)
    mockServerClient.getFilterRuleSuggestions.mockResolvedValueOnce(suggestions)
    jest
      .mocked(fetchOrNull)
      .mockImplementationOnce((fetcher) => fetcher())
      .mockImplementationOnce((fetcher) => fetcher())
    render(await FeedStatsPage())
    expect(fallbackProps[0]).toEqual({
      [swrKeys.feedStats]: stats,
      [swrKeys.feedFilterRuleSuggestions]: suggestions
    })
  })

  it('shows a breadcrumb back to /feeds', async () => {
    render(await FeedStatsPage())
    expect(screen.getByRole('link', { name: 'Feeds' })).toHaveAttribute('href', '/feeds')
    expect(screen.getByText('Stats')).toHaveAttribute('aria-current', 'page')
  })
})
