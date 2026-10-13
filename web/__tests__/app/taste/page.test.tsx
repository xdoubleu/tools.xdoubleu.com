import React from 'react'
import { render, screen } from '@testing-library/react'

const fetchOrNull = jest.fn()
const swrFallback = jest.fn()
const serverClient = {
  getProfile: jest.fn(),
  listRecommendations: jest.fn(),
  getStats: jest.fn(),
  listEvents: jest.fn()
}

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
  it('seeds the profile, the stats and the first log page', async () => {
    const profile = { configured: true }
    const picks = { recommendations: [] }
    const stats = { domains: [] }
    const events = { events: [] }
    fetchOrNull
      .mockResolvedValueOnce(profile)
      .mockResolvedValueOnce(picks)
      .mockResolvedValueOnce(stats)
      .mockResolvedValueOnce(events)
    render(await TastePage({ searchParams: Promise.resolve({}) }))
    expect(screen.getByTestId('taste-client')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Taste' })).toBeInTheDocument()
    expect(serverClient.getProfile).toHaveBeenCalledWith({})
    expect(serverClient.listRecommendations).toHaveBeenCalledWith({ domain: '', track: 'more' })
    expect(serverClient.getStats).toHaveBeenCalledWith({})
    expect(serverClient.listEvents).toHaveBeenCalledWith({ domain: '', limit: 50 })
    expect(swrFallback).toHaveBeenCalledWith(
      expect.objectContaining({
        fallback: { '/taste/profile': profile, '/taste/stats': stats },
        keyed: [
          [['/taste/recommendations', '', 'more'], picks],
          [['/taste/events', ''], events]
        ]
      })
    )
  })

  it('reads the shared taste for ?list=shared', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await TastePage({ searchParams: Promise.resolve({ list: 'shared' }) }))
    expect(serverClient.getProfile).toHaveBeenCalledWith({ shared: true })
    expect(serverClient.listEvents).toHaveBeenCalledWith({
      domain: '',
      limit: 50,
      shared: true
    })
  })

  it('renders without seeding when the fetches fail', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await TastePage({ searchParams: Promise.resolve({}) }))
    expect(swrFallback).toHaveBeenCalledWith(expect.objectContaining({ fallback: {}, keyed: [] }))
  })
})
