import { renderHook } from '@testing-library/react'

const sampleKeys = [['/movies/backlog', '', '', 'added'], '/movies/title/x', '/books', undefined]
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
  addTitle: jest.fn().mockResolvedValue({ entry: { id: 'e-2' } }),
  setStatus: jest.fn().mockResolvedValue({}),
  removeTitle: jest.fn().mockResolvedValue({})
}
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => mockClient)
}))
jest.mock('@/lib/gen/movies/v1/movies_pb', () => ({ MoviesService: {} }))

import useSWR from 'swr'
import {
  useFetchMoviesBacklogPage,
  useMovieSearch,
  useMovieTitle,
  useMoviesActions,
  useMoviesBacklog
} from '@/hooks/useMovies'

const mockUseSWR = jest.mocked(useSWR)
const filter = { status: 'want', mediaType: 'movie', sort: 'title' }

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
      ['/movies/backlog', 'want', 'movie', 'title'],
      expect.any(Function)
    )
    expect(mockClient.listBacklog).toHaveBeenCalledWith({ ...filter, limit: 50 })
  })
})

describe('useFetchMoviesBacklogPage', () => {
  it('fetches a page at an offset', async () => {
    const { result } = renderHook(() => useFetchMoviesBacklogPage(filter))
    await expect(result.current(50)).resolves.toEqual({ items: [{ id: 'e-1' }], hasMore: true })
    expect(mockClient.listBacklog).toHaveBeenCalledWith({ ...filter, limit: 50, offset: 50 })
  })
})

describe('useMovieSearch', () => {
  it('skips queries under two characters', () => {
    renderHook(() => useMovieSearch(' a '))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function), {
      keepPreviousData: true
    })
  })

  it('searches the trimmed query', () => {
    renderHook(() => useMovieSearch(' dune '))
    expect(mockUseSWR).toHaveBeenCalledWith(['/movies/search', 'dune'], expect.any(Function), {
      keepPreviousData: true
    })
    expect(mockClient.searchTitles).toHaveBeenCalledWith({ query: 'dune' })
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
      status: 'want'
    })
    await result.current.setStatus('e-2', 'watched')
    expect(mockClient.setStatus).toHaveBeenCalledWith({ id: 'e-2', status: 'watched' })
    expect(revalidated).toEqual([['/movies/backlog', '', '', 'added'], '/movies/title/x'])
    await result.current.remove('x')
    expect(mockClient.removeTitle).toHaveBeenCalledWith({ id: 'x' })

    expect(mockMutate).toHaveBeenCalledTimes(3)
    // The removed title itself is not revalidated.
    expect(revalidated).toEqual([['/movies/backlog', '', '', 'added']])
  })
})
