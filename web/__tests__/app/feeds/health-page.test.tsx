import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/components/feeds/UnhealthyFeeds', () => () => <div data-testid="unhealthy-feeds" />)

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

const mockFetchOrNull = jest.fn<Promise<unknown>, [() => unknown]>(async () => null)
jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => unknown) => mockFetchOrNull(fn)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({
    children,
    fallback
  }: {
    children: React.ReactNode
    fallback: Record<string, unknown>
  }) => (
    <>
      <div data-testid="swr-fallback-keys">{Object.keys(fallback).join(',')}</div>
      {children}
    </>
  )
}))

import FeedHealthPage from '@/app/feeds/health/page'

describe('FeedHealthPage', () => {
  beforeEach(() => {
    mockFetchOrNull.mockReset().mockResolvedValue(null)
  })

  it('renders the unhealthy feeds client', async () => {
    render(await FeedHealthPage())
    expect(screen.getByTestId('unhealthy-feeds')).toBeInTheDocument()
  })

  it('shows a breadcrumb back to /feeds', async () => {
    render(await FeedHealthPage())
    expect(screen.getByRole('link', { name: 'Feeds' })).toHaveAttribute('href', '/feeds')
    expect(screen.getByText('Health')).toHaveAttribute('aria-current', 'page')
  })

  it('passes the prefetch under the unhealthy-feeds SWR key when the api returns data', async () => {
    mockFetchOrNull.mockResolvedValueOnce({ feeds: [] })
    render(await FeedHealthPage())
    expect(screen.getByTestId('swr-fallback-keys')).toHaveTextContent('/feeds/unhealthy')
  })

  it('passes no fallback data when the api denies the prefetch', async () => {
    render(await FeedHealthPage())
    expect(screen.getByTestId('swr-fallback-keys')).toHaveTextContent('')
  })
})
