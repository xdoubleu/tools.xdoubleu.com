import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/feeds/FeedStatsClient', () => () => <div data-testid="feed-stats-client" />)

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

import FeedStatsPage from '@/app/feeds/stats/page'

describe('FeedStatsPage', () => {
  it('renders the stats client', async () => {
    render(await FeedStatsPage())
    expect(screen.getByTestId('feed-stats-client')).toBeInTheDocument()
  })

  it('links back to /feeds', async () => {
    render(await FeedStatsPage())
    expect(screen.getByRole('link', { name: 'Back to feeds' })).toHaveAttribute('href', '/feeds')
  })
})
