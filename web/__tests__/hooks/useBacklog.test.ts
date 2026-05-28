import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getSteamGame: jest.fn().mockResolvedValue({}),
    getSteamDistribution: jest.fn().mockResolvedValue({}),
    getSteam: jest.fn().mockResolvedValue({}),
    getBooksProgress: jest.fn().mockResolvedValue({}),
    importBooks: jest.fn().mockResolvedValue({})
  }))
}))
jest.mock('@/lib/gen/backlog/v1/books_pb', () => ({ BooksService: {} }))
jest.mock('@/lib/gen/backlog/v1/games_pb', () => ({ GamesService: {} }))

import useSWR from 'swr'
import {
  useBacklogLibrary,
  useBacklogSteam,
  useBacklogSteamGame,
  useBacklogDistribution,
  useBooksProgress,
  useSteamProgress,
  useSearchExternal,
  useAddBook,
  useImportBooks,
  useUpdateBookStatus,
  useToggleTag
} from '@/hooks/useBacklog'
import { createServiceClient } from '@/lib/client'

const mockUseSWR = jest.mocked(useSWR)
const mockCreateServiceClient = jest.mocked(createServiceClient)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
  mockCreateServiceClient.mockClear()
})

describe('useBacklogLibrary', () => {
  it('uses /backlog/books as key', () => {
    renderHook(() => useBacklogLibrary())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/books', expect.any(Function))
  })
})

describe('useBacklogSteam', () => {
  it('uses /backlog/steam as key', () => {
    renderHook(() => useBacklogSteam())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/steam', expect.any(Function))
  })
})

describe('useBacklogSteamGame', () => {
  it('uses correct key when gameId is provided', () => {
    renderHook(() => useBacklogSteamGame(12345))
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/steam/12345', expect.any(Function))
  })

  it('passes null as key when gameId is 0', () => {
    renderHook(() => useBacklogSteamGame(0))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('fetcher calls client.getSteamGame', async () => {
    const mockClient = { getSteamGame: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useBacklogSteamGame(42))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteamGame).toHaveBeenCalledWith({ gameId: 42 })
  })
})

describe('useBacklogDistribution', () => {
  it('uses /backlog/steam/distribution/${bucket} as key', () => {
    renderHook(() => useBacklogDistribution(10))
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/steam/distribution/10', expect.any(Function))
  })

  it('fetcher calls client.getSteamDistribution', async () => {
    const mockClient = { getSteamDistribution: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useBacklogDistribution(10))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteamDistribution).toHaveBeenCalledWith({ bucket: 10 })
  })
})

describe('useSteamProgress', () => {
  it('uses correct key with dates', () => {
    renderHook(() => useSteamProgress('2024-01-01', '2024-12-31'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/backlog/steam/progress', '2024-01-01', '2024-12-31'])
  })

  it('uses key with undefined dates', () => {
    renderHook(() => useSteamProgress())
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/backlog/steam/progress', undefined, undefined])
  })

  it('fetcher calls client.getSteam with dates', async () => {
    const mockClient = { getSteam: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useSteamProgress('2024-01-01', '2024-12-31'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteam).toHaveBeenCalledWith({
      dateStart: '2024-01-01',
      dateEnd: '2024-12-31'
    })
  })
})

describe('useBooksProgress', () => {
  it('uses correct key with date range', () => {
    renderHook(() => useBooksProgress('2024-01-01', '2024-12-31'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/backlog/books/progress', '2024-01-01', '2024-12-31'])
  })

  it('passes null as key when no dates provided', () => {
    renderHook(() => useBooksProgress())
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('fetcher calls client.getBooksProgress', async () => {
    const mockClient = { getBooksProgress: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useBooksProgress('2024-01-01', '2024-12-31'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getBooksProgress).toHaveBeenCalledWith({
      dateStart: '2024-01-01',
      dateEnd: '2024-12-31'
    })
  })
})

describe('useSearchExternal', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useSearchExternal())
    expect(typeof result.current).toBe('function')
  })
})

describe('useAddBook', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useAddBook())
    expect(typeof result.current).toBe('function')
  })
})

describe('useImportBooks', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useImportBooks())
    expect(typeof result.current).toBe('function')
  })

  it('encodes csv and calls client.importBooks', () => {
    const mockImportBooks = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ importBooks: mockImportBooks })
    const { result } = renderHook(() => useImportBooks())
    result.current('a,b\n1,2')
    const call = mockImportBooks.mock.calls[0][0]
    expect(Object.prototype.toString.call(call.csvData)).toBe('[object Uint8Array]')
  })
})

describe('useUpdateBookStatus', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useUpdateBookStatus())
    expect(typeof result.current).toBe('function')
  })
})

describe('useToggleTag', () => {
  it('returns a function', () => {
    const { result } = renderHook(() => useToggleTag())
    expect(typeof result.current).toBe('function')
  })
})
