import { renderHook } from '@testing-library/react'

const mockMutate = jest.fn()
jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  useSWRConfig: () => ({ mutate: mockMutate })
}))
const mockClient = {
  getProfile: jest.fn().mockResolvedValue({}),
  refreshProfile: jest.fn().mockResolvedValue({}),
  getStats: jest.fn().mockResolvedValue({}),
  listEvents: jest.fn().mockResolvedValue({ events: [{ itemId: 'b1' }], hasMore: true })
}
jest.mock('@/lib/client', () => ({ createServiceClient: jest.fn(() => mockClient) }))
jest.mock('@/lib/gen/taste/v1/taste_pb', () => ({ TasteService: {} }))

import useSWR from 'swr'
import {
  PROFILE_POLL_MS,
  useFetchTasteEventsPage,
  useRefreshTasteProfile,
  useTasteEvents,
  useTasteProfile,
  useTasteStats
} from '@/hooks/useTaste'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockImplementation((key: unknown, fetcher: unknown) => {
    if (key !== null && typeof fetcher === 'function') void fetcher()
    return { data: undefined, isLoading: false, error: undefined }
  })
})

describe('useTasteStats', () => {
  it('fetches the stats', () => {
    renderHook(() => useTasteStats())
    expect(mockUseSWR).toHaveBeenCalledWith('/taste/stats', expect.any(Function))
    expect(mockClient.getStats).toHaveBeenCalledWith({})
  })
})

describe('useTasteEvents', () => {
  it('fetches the first page of a domain', () => {
    renderHook(() => useTasteEvents('games'))
    expect(mockUseSWR).toHaveBeenCalledWith(['/taste/events', 'games'], expect.any(Function))
    expect(mockClient.listEvents).toHaveBeenCalledWith({ domain: 'games', limit: 50 })
  })
})

describe('useFetchTasteEventsPage', () => {
  it('fetches a later page as items', async () => {
    const { result } = renderHook(() => useFetchTasteEventsPage('books'))
    await expect(result.current(50)).resolves.toEqual({
      items: [{ itemId: 'b1' }],
      hasMore: true
    })
    expect(mockClient.listEvents).toHaveBeenCalledWith({ domain: 'books', limit: 50, offset: 50 })
  })

  it('follows a domain change', async () => {
    const { result, rerender } = renderHook(({ domain }) => useFetchTasteEventsPage(domain), {
      initialProps: { domain: 'books' }
    })
    const first = result.current
    rerender({ domain: 'games' })
    expect(result.current).not.toBe(first)
    await result.current(0)
    expect(mockClient.listEvents).toHaveBeenLastCalledWith({
      domain: 'games',
      limit: 50,
      offset: 0
    })
  })
})

describe('useTasteProfile', () => {
  it('fetches the profile and polls only while pending', () => {
    renderHook(() => useTasteProfile())
    expect(mockClient.getProfile).toHaveBeenCalledWith({})
    const [key, , options] = mockUseSWR.mock.calls[0] as unknown as [
      string,
      unknown,
      { refreshInterval: (data?: { pending: boolean }) => number }
    ]
    expect(key).toBe('/taste/profile')
    expect(options.refreshInterval({ pending: true })).toBe(PROFILE_POLL_MS)
    expect(options.refreshInterval({ pending: false })).toBe(0)
    expect(options.refreshInterval(undefined)).toBe(0)
  })
})

describe('useRefreshTasteProfile', () => {
  it('queues a refresh, then re-reads the profile', async () => {
    const { result } = renderHook(() => useRefreshTasteProfile())
    await result.current()
    expect(mockClient.refreshProfile).toHaveBeenCalledWith({})
    expect(mockMutate).toHaveBeenCalledWith('/taste/profile')
  })

  it('re-reads nothing when the refresh is refused', async () => {
    mockClient.refreshProfile.mockRejectedValueOnce(new Error('too soon'))
    const { result } = renderHook(() => useRefreshTasteProfile())
    await expect(result.current()).rejects.toThrow('too soon')
    expect(mockMutate).not.toHaveBeenCalled()
  })
})
