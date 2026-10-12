import React from 'react'
import { render, screen } from '@testing-library/react'

const fetchOrNull = jest.fn()
const swrFallback = jest.fn()
const serverClient = { getStats: jest.fn(), listEvents: jest.fn() }

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => serverClient)
}))

// Runs the fetch so the request arguments are checked.
jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: async (fn: () => Promise<unknown>) => {
    await fn()
    return fetchOrNull(fn)
  }
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: (props: { children: React.ReactNode }) => {
    swrFallback(props)
    return <>{props.children}</>
  }
}))

jest.mock('@/components/taste/TasteClient', () => ({
  __esModule: true,
  default: () => <div data-testid="taste-client" />
}))

import TastePage from '@/app/taste/page'

beforeEach(() => jest.clearAllMocks())

describe('TastePage', () => {
  it('seeds the stats and the first log page', async () => {
    const stats = { domains: [] }
    const events = { events: [] }
    fetchOrNull.mockResolvedValueOnce(stats).mockResolvedValueOnce(events)
    render(await TastePage())
    expect(screen.getByTestId('taste-client')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Taste' })).toBeInTheDocument()
    expect(serverClient.getStats).toHaveBeenCalledWith({})
    expect(serverClient.listEvents).toHaveBeenCalledWith({ domain: '', limit: 50 })
    expect(swrFallback).toHaveBeenCalledWith(
      expect.objectContaining({
        fallback: { '/taste/stats': stats },
        keyed: [[['/taste/events', ''], events]]
      })
    )
  })

  it('renders without seeding when the fetches fail', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await TastePage())
    expect(swrFallback).toHaveBeenCalledWith(expect.objectContaining({ fallback: {}, keyed: [] }))
  })
})
