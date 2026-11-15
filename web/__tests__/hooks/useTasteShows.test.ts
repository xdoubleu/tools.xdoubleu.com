import { renderHook } from '@testing-library/react'

const sampleKeys = [
  '/taste/shows',
  ['/taste/shows/search', 'x'],
  '/taste/stats',
  ['/taste/events', 'books'],
  '/taste/profile',
  '/movies/settings',
  undefined
]
let revalidated: unknown[] = []
const mockMutate = vi.fn(async (match: (key: unknown) => boolean) => {
  revalidated = sampleKeys.filter(match)
})
vi.mock('swr', () => ({
  __esModule: true,
  default: vi.fn(),
  useSWRConfig: () => ({ mutate: mockMutate })
}))
const mockClient = {
  listFavouriteShows: vi.fn().mockResolvedValue({ favourites: [] }),
  searchShows: vi.fn().mockResolvedValue({ results: [] }),
  addFavouriteShow: vi.fn().mockResolvedValue({}),
  removeFavouriteShow: vi.fn().mockResolvedValue({})
}
vi.mock('@/lib/client', () => ({
  createServiceClient: vi.fn(() => mockClient)
}))
vi.mock('@/lib/gen/taste/v1/taste_pb', () => ({ TasteService: {} }))

import useSWR from 'swr'
import { useShowActions, useShowFavourites, useShowSearch } from '@/hooks/useTasteShows'

beforeEach(() => {
  vi.clearAllMocks()
  revalidated = []
})

const mockUseSWR = vi.mocked(useSWR)

beforeEach(() => {
  // Runs the fetcher for a non-null key, like SWR would.
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockImplementation((key: unknown, fetcher: unknown) => {
    if (key !== null && typeof fetcher === 'function') void fetcher()
    return { data: undefined, isLoading: false, error: undefined }
  })
})

describe('useShowFavourites', () => {
  it('fetches the favourites', () => {
    renderHook(() => useShowFavourites())
    expect(mockUseSWR).toHaveBeenCalledWith('/taste/shows', expect.any(Function))
    expect(mockClient.listFavouriteShows).toHaveBeenCalledWith({})
  })
})

describe('useShowSearch', () => {
  it('searches from two characters', () => {
    renderHook(() => useShowSearch(' hi '))
    expect(mockUseSWR).toHaveBeenCalledWith(['/taste/shows/search', 'hi'], expect.any(Function), {
      keepPreviousData: true
    })
    expect(mockClient.searchShows).toHaveBeenCalledWith({ query: 'hi' })
  })

  it('does not fetch for a short query', () => {
    renderHook(() => useShowSearch(' h '))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function), expect.anything())
    expect(mockClient.searchShows).not.toHaveBeenCalled()
  })
})

describe('useShowActions', () => {
  it('adds and revalidates the shows and the taste log, not the profile', async () => {
    const { result } = renderHook(() => useShowActions())
    await result.current.add(7n)
    expect(mockClient.addFavouriteShow).toHaveBeenCalledWith({ itunesId: 7n })
    expect(revalidated).toEqual([
      '/taste/shows',
      ['/taste/shows/search', 'x'],
      '/taste/stats',
      ['/taste/events', 'books']
    ])
  })

  it('removes and revalidates', async () => {
    const { result } = renderHook(() => useShowActions())
    await result.current.remove('f-1')
    expect(mockClient.removeFavouriteShow).toHaveBeenCalledWith({ id: 'f-1' })
    expect(revalidated).toHaveLength(4)
  })
})
