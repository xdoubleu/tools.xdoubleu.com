import React from 'react'
import { render, screen } from '@testing-library/react'

const fetchOrNull = jest.fn()
const swrFallback = jest.fn()
const serverClient = { listBacklog: jest.fn(), getTitle: jest.fn() }

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
  default: (props: { children: React.ReactNode; keyed?: unknown; fallback: unknown }) => {
    swrFallback(props)
    return <>{props.children}</>
  }
}))

jest.mock('@/components/movies/MoviesClient', () => ({
  __esModule: true,
  default: () => <div data-testid="movies-client" />
}))

jest.mock('@/components/movies/MovieTitleClient', () => ({
  __esModule: true,
  default: ({ id }: { id: string }) => <div data-testid="title-client">{id}</div>
}))

import MoviesPage from '@/app/movies/page'
import MovieTitlePage from '@/app/movies/[id]/page'

beforeEach(() => jest.clearAllMocks())

describe('MoviesPage', () => {
  it('seeds the default backlog key', async () => {
    const backlog = { entries: [] }
    fetchOrNull.mockResolvedValue(backlog)
    render(await MoviesPage())
    expect(screen.getByTestId('movies-client')).toBeInTheDocument()
    expect(serverClient.listBacklog).toHaveBeenCalledWith({
      status: '',
      mediaType: '',
      sort: 'added',
      limit: 50
    })
    expect(swrFallback).toHaveBeenCalledWith(
      expect.objectContaining({ keyed: [[['/movies/backlog', '', '', 'added'], backlog]] })
    )
  })

  it('renders without seeding when the fetch fails', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await MoviesPage())
    expect(swrFallback).toHaveBeenCalledWith(expect.objectContaining({ keyed: [] }))
  })
})

describe('MovieTitlePage', () => {
  it('seeds the title key', async () => {
    const title = { entry: { id: 't-1' } }
    fetchOrNull.mockResolvedValue(title)
    render(await MovieTitlePage({ params: Promise.resolve({ id: 't-1' }) }))
    expect(screen.getByTestId('title-client')).toHaveTextContent('t-1')
    expect(serverClient.getTitle).toHaveBeenCalledWith({ id: 't-1' })
    expect(swrFallback).toHaveBeenCalledWith(
      expect.objectContaining({ fallback: { '/movies/title/t-1': title } })
    )
  })

  it('renders without seeding when the fetch fails', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await MovieTitlePage({ params: Promise.resolve({ id: 't-1' }) }))
    expect(swrFallback).toHaveBeenCalledWith(expect.objectContaining({ fallback: {} }))
  })
})
