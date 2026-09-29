import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/feeds/FeedReaderClient', () => () => <div data-testid="feed-reader" />)

jest.mock('@/components/feeds/FeedsHeader', () => () => <div data-testid="feeds-header" />)

jest.mock('@/components/feeds/FeedRuleSuggestionsCard', () => () => (
  <div data-testid="feed-rule-suggestions" />
))

const mockServerClient = {
  listFeedItems: jest.fn(),
  listFeeds: jest.fn(),
  getFilterRuleSuggestions: jest.fn()
}
jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => mockServerClient)
}))

const mockFetchOrNull = jest.fn<Promise<unknown>, [() => unknown]>(async () => null)
jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => unknown) => mockFetchOrNull(fn)
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

import FeedsPage from '@/app/feeds/page'
import { swrKeys } from '@/lib/swrKeys'

describe('FeedsPage', () => {
  beforeEach(() => {
    mockFetchOrNull.mockReset().mockResolvedValue(null)
    fallbackProps.length = 0
  })

  it('renders the filter rule suggestions', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('feed-rule-suggestions')).toBeInTheDocument()
  })

  it('renders the feeds header', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('feeds-header')).toBeInTheDocument()
  })

  it('renders the feed reader', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('feed-reader')).toBeInTheDocument()
  })

  it('renders no link back to /books', async () => {
    render(await FeedsPage())
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })

  it('prefetches feed items, feeds and filter rule suggestions', async () => {
    const items = { items: [] }
    const feeds = { feeds: [] }
    const suggestions = { suggestions: [] }
    mockServerClient.listFeedItems.mockResolvedValueOnce(items)
    mockServerClient.listFeeds.mockResolvedValueOnce(feeds)
    mockServerClient.getFilterRuleSuggestions.mockResolvedValueOnce(suggestions)
    mockFetchOrNull.mockImplementation(async (fetcher) => fetcher())
    render(await FeedsPage())
    expect(fallbackProps[0]).toEqual({
      [swrKeys.feedItems(true)]: items,
      [swrKeys.feeds]: feeds,
      [swrKeys.feedFilterRuleSuggestions]: suggestions
    })
  })
})
