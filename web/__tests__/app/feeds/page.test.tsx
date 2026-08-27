import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/feeds/FeedReaderClient', () => () => <div data-testid="feed-reader" />)

jest.mock('@/components/feeds/FeedsHeader', () => () => <div data-testid="feeds-header" />)

jest.mock('@/components/feeds/UnhealthyFeedsSection', () => () => (
  <div data-testid="unhealthy-feeds-section" />
))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

const mockFetchOrNull = jest.fn<Promise<unknown>, [() => unknown]>(async () => null)
jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => unknown) => mockFetchOrNull(fn)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import FeedsPage from '@/app/feeds/page'

describe('FeedsPage', () => {
  beforeEach(() => {
    mockFetchOrNull.mockReset().mockResolvedValue(null)
  })

  it('renders the feeds header', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('feeds-header')).toBeInTheDocument()
  })

  it('renders the feed reader', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('feed-reader')).toBeInTheDocument()
  })

  it('renders the unhealthy feeds section', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('unhealthy-feeds-section')).toBeInTheDocument()
  })

  it('renders no link back to /books', async () => {
    render(await FeedsPage())
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })

  it('renders when the server prefetch returns data for both feed items and feeds', async () => {
    mockFetchOrNull.mockResolvedValueOnce({ items: [] }).mockResolvedValueOnce({ feeds: [] })
    render(await FeedsPage())
    expect(screen.getByTestId('feed-reader')).toBeInTheDocument()
  })
})
