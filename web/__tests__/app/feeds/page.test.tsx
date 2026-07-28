import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/reading/FeedReaderClient', () => () => <div data-testid="feed-reader" />)

jest.mock('@/components/reading/FeedsHeader', () => () => <div data-testid="feeds-header" />)

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import FeedsPage from '@/app/feeds/page'

describe('FeedsPage', () => {
  it('renders the feeds header', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('feeds-header')).toBeInTheDocument()
  })

  it('renders the feed reader', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('feed-reader')).toBeInTheDocument()
  })

  it('renders no link back to /reading', async () => {
    render(await FeedsPage())
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })
})
