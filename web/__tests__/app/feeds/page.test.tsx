import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/reading/FeedReaderClient', () => () => <div data-testid="feed-reader" />)

jest.mock('@/components/reading/ManageFeedsSection', () => () => <div data-testid="manage-feeds" />)

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
  it('renders the Feeds heading', async () => {
    render(await FeedsPage())
    expect(screen.getByRole('heading', { name: 'Feeds' })).toBeInTheDocument()
  })

  it('renders a link back to /reading', async () => {
    render(await FeedsPage())
    expect(screen.getByRole('link', { name: /reading/i })).toHaveAttribute('href', '/reading')
  })

  it('renders the feed reader', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('feed-reader')).toBeInTheDocument()
  })

  it('renders the manage feeds section', async () => {
    render(await FeedsPage())
    expect(screen.getByTestId('manage-feeds')).toBeInTheDocument()
  })
})
