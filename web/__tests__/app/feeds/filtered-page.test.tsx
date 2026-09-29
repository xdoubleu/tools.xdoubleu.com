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

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async (fn: () => unknown) => {
    fn()
    return null
  })
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import FeedFilteredPage from '@/app/feeds/filtered/page'

describe('FeedFilteredPage', () => {
  beforeEach(() => jest.clearAllMocks())

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
    expect(screen.getByRole('link', { name: 'Feeds' })).toHaveAttribute('href', '/feeds')
    expect(screen.getByRole('heading', { name: 'Filtered items' })).toBeInTheDocument()
  })
})
