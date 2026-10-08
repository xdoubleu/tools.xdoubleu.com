import { renderHook } from '@testing-library/react'

const sampleKeys = [
  ['/movies/backlog', '', '', 'added', false],
  '/movies/settings',
  '/movies/title/x',
  '/books',
  undefined
]
let revalidated: unknown[] = []
const mockMutate = jest.fn(async (match: (key: unknown) => boolean) => {
  revalidated = sampleKeys.filter(match)
})
jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  useSWRConfig: () => ({ mutate: mockMutate })
}))
const mockClient = {
  listBacklog: jest.fn().mockResolvedValue({ entries: [{ id: 'e-1' }], hasMore: true }),
  searchTitles: jest.fn().mockResolvedValue({ results: [] }),
  getTitle: jest.fn().mockResolvedValue({}),
  getStats: jest.fn().mockResolvedValue({}),
  getSettings: jest.fn().mockResolvedValue({ providerIds: [] }),
  setSettings: jest.fn().mockResolvedValue({ providerIds: [] }),
  listAvailableProviders: jest.fn().mockResolvedValue({ providers: [] }),
  addTitle: jest.fn().mockResolvedValue({ entry: { id: 'e-2' } }),
  setStatus: jest.fn().mockResolvedValue({}),
  removeTitle: jest.fn().mockResolvedValue({}),
  setRating: jest.fn().mockResolvedValue({}),
  setSeasonWatched: jest.fn().mockResolvedValue({}),
  addWatchDate: jest.fn().mockResolvedValue({}),
  editWatchDate: jest.fn().mockResolvedValue({}),
  removeWatchDate: jest.fn().mockResolvedValue({})
}
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => mockClient)
}))
jest.mock('@/lib/gen/movies/v1/movies_pb', () => ({ MoviesService: {} }))

import useSWR from 'swr'
import {
  useFetchMoviesBacklogPage,
  useAvailableProviders,
  useMovieSearch,
  useMovieSettings,
  useMovieStats,
  useMovieTitle,
  useMoviesActions,
  useMoviesBacklog
} from '@/hooks/useMovies'

const mockUseSWR = jest.mocked(useSWR)
const filter = { status: 'want', mediaType: 'movie', sort: 'title', onMyServices: false }

beforeEach(() => {
  jest.clearAllMocks()
  // Runs the fetcher for a non-null key, like SWR would.
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockImplementation((key: unknown, fetcher: unknown) => {
    if (key !== null && typeof fetcher === 'function') void fetcher()
    return { data: undefined, isLoading: false, error: undefined }
  })
})

describe('useMoviesBacklog', () => {
  it('keys by filter and fetches the first page', () => {
    renderHook(() => useMoviesBacklog(filter))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/movies/backlog', 'want', 'movie', 'title', false],
      expect.any(Function)
    )
    expect(mockClient.listBacklog).toHaveBeenCalledWith({ ...filter, newSeason: false, limit: 50 })
  })

  it('asks for new seasons on the New seasons tab', () => {
    renderHook(() => useMoviesBacklog({ ...filter, status: 'new' }))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/movies/backlog', 'new', 'movie', 'title', false],
      expect.any(Function)
    )
    expect(mockClient.listBacklog).toHaveBeenCalledWith({
      ...filter,
      status: '',
      newSeason: true,
      limit: 50
    })
  })
})

describe('useFetchMoviesBacklogPage', () => {
  it('fetches a page at an offset', async () => {
    const { result } = renderHook(() => useFetchMoviesBacklogPage(filter))
    await expect(result.current(50)).resolves.toEqual({ items: [{ id: 'e-1' }], hasMore: true })
    expect(mockClient.listBacklog).toHaveBeenCalledWith({
      ...filter,
      newSeason: false,
      limit: 50,
      offset: 50
    })
  })

  it('follows filter changes', async () => {
    const { result, rerender } = renderHook((f: typeof filter) => useFetchMoviesBacklogPage(f), {
      initialProps: filter
    })
    rerender({ ...filter, status: 'new' })
    await result.current(0)
    expect(mockClient.listBacklog).toHaveBeenLastCalledWith({
      ...filter,
      status: '',
      newSeason: true,
      limit: 50,
      offset: 0
    })
  })
})

describe('useMovieSearch', () => {
  it('skips queries under two characters', () => {
    renderHook(() => useMovieSearch(' a '))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function), {
      keepPreviousData: true
    })
  })

  it('searches from two characters', () => {
    renderHook(() => useMovieSearch('ab'))
    expect(mockClient.searchTitles).toHaveBeenCalledWith({ query: 'ab' })
  })

  it('searches the trimmed query', () => {
    renderHook(() => useMovieSearch(' dune '))
    expect(mockUseSWR).toHaveBeenCalledWith(['/movies/search', 'dune'], expect.any(Function), {
      keepPreviousData: true
    })
    expect(mockClient.searchTitles).toHaveBeenCalledWith({ query: 'dune' })
  })
})

describe('useMovieStats', () => {
  it('fetches the stats', () => {
    renderHook(() => useMovieStats())
    expect(mockUseSWR).toHaveBeenCalledWith('/movies/stats', expect.any(Function))
    expect(mockClient.getStats).toHaveBeenCalledWith({})
  })
})

describe('useMovieSettings', () => {
  it('fetches the picked services', () => {
    renderHook(() => useMovieSettings())
    expect(mockUseSWR).toHaveBeenCalledWith('/movies/settings', expect.any(Function))
    expect(mockClient.getSettings).toHaveBeenCalledWith({})
  })
})

describe('useAvailableProviders', () => {
  it('fetches the Belgian providers', () => {
    renderHook(() => useAvailableProviders())
    expect(mockUseSWR).toHaveBeenCalledWith('/movies/providers', expect.any(Function))
    expect(mockClient.listAvailableProviders).toHaveBeenCalledWith({})
  })
})

describe('useMovieTitle', () => {
  it('keys by id, or null without one', () => {
    renderHook(() => useMovieTitle('t-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/movies/title/t-1', expect.any(Function))
    expect(mockClient.getTitle).toHaveBeenCalledWith({ id: 't-1' })

    renderHook(() => useMovieTitle(''))
    expect(mockUseSWR).toHaveBeenLastCalledWith(null, expect.any(Function))
  })
})

describe('useMoviesActions', () => {
  it('writes, then revalidates only movies keys', async () => {
    const { result } = renderHook(() => useMoviesActions())

    await expect(result.current.add('movie', 603n, 'want')).resolves.toEqual({ id: 'e-2' })
    expect(mockClient.addTitle).toHaveBeenCalledWith({
      mediaType: 'movie',
      tmdbId: 603n,
      status: 'want',
      unknownDate: false
    })
    await result.current.add('series', 1n, 'watched', true)
    expect(mockClient.addTitle).toHaveBeenLastCalledWith({
      mediaType: 'series',
      tmdbId: 1n,
      status: 'watched',
      unknownDate: true
    })
    await result.current.setStatus('e-2', 'watched')
    expect(mockClient.setStatus).toHaveBeenCalledWith({
      id: 'e-2',
      status: 'watched',
      unknownDate: false
    })
    await result.current.setStatus('e-2', 'watched', true)
    expect(mockClient.setStatus).toHaveBeenLastCalledWith({
      id: 'e-2',
      status: 'watched',
      unknownDate: true
    })
    expect(revalidated).toEqual([
      ['/movies/backlog', '', '', 'added', false],
      '/movies/settings',
      '/movies/title/x'
    ])
    await result.current.remove('x')
    expect(mockClient.removeTitle).toHaveBeenCalledWith({ id: 'x' })

    expect(mockMutate).toHaveBeenCalledTimes(5)
    // The removed title itself is not revalidated.
    expect(revalidated).toEqual([['/movies/backlog', '', '', 'added', false], '/movies/settings'])
  })

  it('saves the picked services, then revalidates movies keys', async () => {
    const { result } = renderHook(() => useMoviesActions())

    await result.current.setServices([8n, 119n])

    expect(mockClient.setSettings).toHaveBeenCalledWith({ providerIds: [8n, 119n] })
    expect(revalidated).toEqual([
      ['/movies/backlog', '', '', 'added', false],
      '/movies/settings',
      '/movies/title/x'
    ])
  })

  it('rates and clears a rating, then revalidates', async () => {
    const { result } = renderHook(() => useMoviesActions())

    await result.current.setRating('e-1', 4)
    expect(mockClient.setRating).toHaveBeenCalledWith({ id: 'e-1', rating: 4 })
    await result.current.setRating('e-1', 0)
    expect(mockClient.setRating).toHaveBeenLastCalledWith({ id: 'e-1', rating: undefined })
    expect(mockMutate).toHaveBeenCalledTimes(2)
  })

  it('writes seasons and watch dates, then revalidates', async () => {
    const { result } = renderHook(() => useMoviesActions())

    await result.current.setSeasonWatched('e-1', 2, true)
    expect(mockClient.setSeasonWatched).toHaveBeenCalledWith({
      id: 'e-1',
      seasonNumber: 2,
      watched: true,
      unknownDate: false
    })
    await result.current.addWatchDate('e-1', 2, '')
    expect(mockClient.addWatchDate).toHaveBeenCalledWith({ id: 'e-1', seasonNumber: 2, date: '' })
    await result.current.editWatchDate('e-1', undefined, 1, '2020-01-02')
    expect(mockClient.editWatchDate).toHaveBeenCalledWith({
      id: 'e-1',
      seasonNumber: undefined,
      index: 1,
      date: '2020-01-02'
    })
    await result.current.removeWatchDate('e-1', 3, 0)
    expect(mockClient.removeWatchDate).toHaveBeenCalledWith({
      id: 'e-1',
      seasonNumber: 3,
      index: 0
    })
    expect(mockMutate).toHaveBeenCalledTimes(4)
    expect(revalidated).toEqual([
      ['/movies/backlog', '', '', 'added', false],
      '/movies/settings',
      '/movies/title/x'
    ])
  })
})
