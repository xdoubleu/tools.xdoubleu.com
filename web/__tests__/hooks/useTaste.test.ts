import { renderHook } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

let mockMutate = jest.fn()
jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  useSWRConfig: () => ({ mutate: mockMutate })
}))
const mockClient = {
  getProfile: jest.fn().mockResolvedValue({}),
  listRecommendations: jest.fn().mockResolvedValue({}),
  refreshProfile: jest.fn().mockResolvedValue({}),
  refreshRecommendations: jest.fn().mockResolvedValue({}),
  getStats: jest.fn().mockResolvedValue({}),
  listEvents: jest.fn().mockResolvedValue({ events: [{ itemId: 'b1' }], hasMore: true }),
  setFeedback: jest.fn().mockResolvedValue({}),
  setSaved: jest.fn().mockResolvedValue({}),
  acceptRecommendation: jest.fn().mockResolvedValue({}),
  setEmailNewPicks: jest.fn().mockResolvedValue({})
}
jest.mock('@/lib/client', () => ({ createServiceClient: jest.fn(() => mockClient) }))
jest.mock('@/lib/gen/taste/v1/taste_pb', () => ({
  ...jest.requireActual('@/lib/gen/taste/v1/taste_pb'),
  TasteService: {}
}))

import useSWR from 'swr'
import { GetProfileResponseSchema } from '@/lib/gen/taste/v1/taste_pb'
import {
  PENDING_POLL_MS,
  isRecommendationsKey,
  pendingRefreshInterval,
  useFetchTasteEventsPage,
  useRefreshTastePicks,
  useRefreshTasteProfile,
  useTasteEvents,
  useTasteProfile,
  usePickActions,
  useSetEmailNewPicks,
  useTasteRecommendations,
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
  it('fetches the profile, re-reading on its own schedule', () => {
    renderHook(() => useTasteProfile())
    expect(mockClient.getProfile).toHaveBeenCalledWith({})
    expect(mockUseSWR).toHaveBeenCalledWith('/taste/profile', expect.any(Function), {
      refreshInterval: pendingRefreshInterval
    })
  })
})

describe('pendingRefreshInterval', () => {
  const at = (ms: number) => new Date(Date.now() + ms).toISOString()
  const profile = (pending: boolean, nextRefreshAt: string) =>
    create(GetProfileResponseSchema, { pending, nextRefreshAt })

  it('polls while pending', () => {
    expect(pendingRefreshInterval(profile(true, at(3_600_000)))).toBe(PENDING_POLL_MS)
  })

  it('re-reads once the cooldown ends', () => {
    const delay = pendingRefreshInterval(profile(false, at(3_600_000)))
    expect(delay).toBeGreaterThan(3_590_000)
    expect(delay).toBeLessThanOrEqual(3_600_000)
  })

  it('waits at least the poll interval for a cooldown about to end', () => {
    expect(pendingRefreshInterval(profile(false, at(1_000)))).toBe(PENDING_POLL_MS)
    expect(pendingRefreshInterval(profile(false, at(-60_000)))).toBe(PENDING_POLL_MS)
  })

  it('stops without a cooldown or data', () => {
    expect(pendingRefreshInterval(profile(false, ''))).toBe(0)
    expect(pendingRefreshInterval(undefined)).toBe(0)
  })
})

describe('useRefreshTasteProfile', () => {
  it('queues a refresh, then re-reads the profile', async () => {
    const { result } = renderHook(() => useRefreshTasteProfile())
    await result.current()
    expect(mockClient.refreshProfile).toHaveBeenCalledWith({})
    expect(mockMutate).toHaveBeenCalledWith('/taste/profile')
    expect(mockMutate).toHaveBeenCalledWith(isRecommendationsKey)
  })

  it('uses the current SWR mutate', async () => {
    const { result, rerender } = renderHook(() => useRefreshTasteProfile())
    const first = mockMutate
    mockMutate = jest.fn()
    rerender()
    await result.current()
    expect(mockMutate).toHaveBeenCalledWith('/taste/profile')
    expect(first).not.toHaveBeenCalled()
  })

  it('re-reads nothing when the refresh is refused', async () => {
    mockClient.refreshProfile.mockRejectedValueOnce(new Error('too soon'))
    const { result } = renderHook(() => useRefreshTasteProfile())
    await expect(result.current()).rejects.toThrow('too soon')
    expect(mockMutate).not.toHaveBeenCalled()
  })
})

describe('useTasteRecommendations', () => {
  it('fetches a domain and track', () => {
    renderHook(() => useTasteRecommendations('books', 'broaden'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/taste/recommendations', 'books', 'broaden'],
      expect.any(Function),
      { refreshInterval: pendingRefreshInterval }
    )
    expect(mockClient.listRecommendations).toHaveBeenCalledWith({
      domain: 'books',
      track: 'broaden',
      savedOnly: false
    })
  })

  it('fetches every saved pick for the saved view', () => {
    renderHook(() => useTasteRecommendations('', 'saved'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/taste/recommendations', '', 'saved'],
      expect.any(Function),
      { refreshInterval: pendingRefreshInterval }
    )
    expect(mockClient.listRecommendations).toHaveBeenCalledWith({
      domain: '',
      track: '',
      savedOnly: true
    })
  })
})

describe('usePickActions', () => {
  it('writes, then re-reads the picks and the stats', async () => {
    const { result } = renderHook(() => usePickActions())
    await result.current.setFeedback('r1', 'hit')
    expect(mockClient.setFeedback).toHaveBeenCalledWith({ id: 'r1', feedback: 'hit' })
    await result.current.setSaved('r1', true)
    expect(mockClient.setSaved).toHaveBeenCalledWith({ id: 'r1', saved: true })
    await result.current.accept('r1')
    expect(mockClient.acceptRecommendation).toHaveBeenCalledWith({ id: 'r1' })
    expect(mockMutate).toHaveBeenCalledWith(isRecommendationsKey)
    expect(mockMutate).toHaveBeenCalledWith('/taste/stats')
    expect(mockMutate).toHaveBeenCalledTimes(6)
  })

  it('uses the current mutate', async () => {
    const { result, rerender } = renderHook(() => usePickActions())
    const first = mockMutate
    mockMutate = jest.fn()
    rerender()
    await result.current.setSaved('r1', true)
    expect(mockMutate).toHaveBeenCalled()
    expect(first).not.toHaveBeenCalled()
    mockMutate = first
  })

  it('re-reads nothing when the write fails', async () => {
    mockClient.setSaved.mockRejectedValueOnce(new Error('nope'))
    const { result } = renderHook(() => usePickActions())
    await expect(result.current.setSaved('r1', false)).rejects.toThrow('nope')
    expect(mockMutate).not.toHaveBeenCalled()
  })
})

describe('isRecommendationsKey', () => {
  it('matches every recommendations key only', () => {
    expect(isRecommendationsKey(['/taste/recommendations', 'books', 'more'])).toBe(true)
    expect(isRecommendationsKey(['/taste/events', ''])).toBe(false)
    expect(isRecommendationsKey('/taste/recommendations')).toBe(false)
  })
})

describe('useRefreshTastePicks', () => {
  it('queues new picks, then re-reads them', async () => {
    const { result } = renderHook(() => useRefreshTastePicks())
    await result.current()
    expect(mockClient.refreshRecommendations).toHaveBeenCalledWith({})
    expect(mockMutate).toHaveBeenCalledWith(isRecommendationsKey)
    expect(mockMutate).not.toHaveBeenCalledWith('/taste/profile')
  })

  it('re-reads nothing when the refresh is refused', async () => {
    mockClient.refreshRecommendations.mockRejectedValueOnce(new Error('too soon'))
    const { result } = renderHook(() => useRefreshTastePicks())
    await expect(result.current()).rejects.toThrow('too soon')
    expect(mockMutate).not.toHaveBeenCalled()
  })
})

describe('useSetEmailNewPicks', () => {
  it('writes the setting, then re-reads the picks', async () => {
    const { result } = renderHook(() => useSetEmailNewPicks())
    await result.current(false)
    expect(mockClient.setEmailNewPicks).toHaveBeenCalledWith({ enabled: false })
    expect(mockMutate).toHaveBeenCalledWith(isRecommendationsKey)
  })

  it('re-reads nothing when the write fails', async () => {
    mockClient.setEmailNewPicks.mockRejectedValueOnce(new Error('nope'))
    const { result } = renderHook(() => useSetEmailNewPicks())
    await expect(result.current(true)).rejects.toThrow('nope')
    expect(mockMutate).not.toHaveBeenCalled()
  })
})
