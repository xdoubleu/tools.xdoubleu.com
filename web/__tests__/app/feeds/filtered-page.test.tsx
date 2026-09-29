import React from 'react'
import { render, screen } from '@testing-library/react'

const mockClient = jest.fn((props: { initialFeedId?: string }) => (
  <div data-testid="filtered-client">{props.initialFeedId ?? 'all'}</div>
))
jest.mock('@/components/feeds/FeedFilteredItemsClient', () => ({
  __esModule: true,
  default: (props: { initialFeedId?: string }) => mockClient(props)
}))

const listFeedItems = jest.fn()
const listFeeds = jest.fn()
jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({ listFeedItems, listFeeds }))
}))

let mockFetched: unknown = null
jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async (fn: () => unknown) => {
    fn()
    return mockFetched
  })
}))

const mockFallback = jest.fn()
jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({
    children,
    fallback
  }: {
    children: React.ReactNode
    fallback: Record<string, unknown>
  }) => {
    mockFallback(fallback)
    return <>{children}</>
  }
}))

import FeedFilteredPage from '@/app/feeds/filtered/page'
import { swrKeys } from '@/lib/swrKeys'

describe('FeedFilteredPage', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockFetched = null
  })

  it('seeds the SWR cache under the keys the client reads', async () => {
    mockFetched = { items: [] }
    render(await FeedFilteredPage({ searchParams: Promise.resolve({ feed: 'feed-1' }) }))
    expect(mockFallback).toHaveBeenCalledWith({
      [swrKeys.feedFilteredItems('feed-1')]: mockFetched,
      [swrKeys.feeds]: mockFetched
    })
  })

  it('prefetches filtered items for the requested feed and opens on it', async () => {
    render(await FeedFilteredPage({ searchParams: Promise.resolve({ feed: 'feed-1' }) }))
    expect(listFeedItems).toHaveBeenCalledWith(
      expect.objectContaining({ filteredOnly: true, feedId: 'feed-1' })
    )
    expect(screen.getByTestId('filtered-client')).toHaveTextContent('feed-1')
  })

  it('lists every feed without a feed param and links back to /feeds', async () => {
    render(await FeedFilteredPage({ searchParams: Promise.resolve({}) }))
    expect(listFeedItems).toHaveBeenCalledWith(
      expect.objectContaining({ filteredOnly: true, feedId: undefined })
    )
    expect(screen.getByTestId('filtered-client')).toHaveTextContent('all')
    expect(listFeeds).toHaveBeenCalledWith({})
    expect(screen.getByRole('link', { name: 'Feeds' })).toHaveAttribute('href', '/feeds')
    expect(screen.getByText('Filtered')).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('heading', { name: 'Filtered items' })).toBeInTheDocument()
  })
})
