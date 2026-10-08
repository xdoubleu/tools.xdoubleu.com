import React from 'react'
import { render, screen } from '@testing-library/react'

const fetchOrNull = jest.fn()
const swrFallback = jest.fn()
const serverClient = { listFavourites: jest.fn() }

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
  default: (props: { children: React.ReactNode; keyed?: unknown }) => {
    swrFallback(props)
    return <>{props.children}</>
  }
}))

jest.mock('@/components/podcasts/PodcastsClient', () => ({
  __esModule: true,
  default: () => <div data-testid="podcasts-client" />
}))

import PodcastsPage from '@/app/podcasts/page'

beforeEach(() => jest.clearAllMocks())

describe('PodcastsPage', () => {
  it('seeds the favourites key', async () => {
    const favourites = { favourites: [] }
    fetchOrNull.mockResolvedValue(favourites)
    render(await PodcastsPage())
    expect(screen.getByTestId('podcasts-client')).toBeInTheDocument()
    expect(serverClient.listFavourites).toHaveBeenCalledWith({})
    expect(swrFallback).toHaveBeenCalledWith(
      expect.objectContaining({ keyed: [['/podcasts/favourites', favourites]] })
    )
  })

  it('renders without seeding when the fetch fails', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await PodcastsPage())
    expect(swrFallback).toHaveBeenCalledWith(expect.objectContaining({ keyed: [] }))
  })
})
