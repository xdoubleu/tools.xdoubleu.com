import { renderHook } from '@testing-library/react'

const mutateMock = jest.fn()
jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  mutate: (...args: unknown[]) => mutateMock(...args)
}))

const clientMocks = {
  listFeeds: jest.fn().mockResolvedValue({ feeds: [] }),
  createFeed: jest.fn().mockResolvedValue({ ingested: 2 }),
  updateFeed: jest.fn().mockResolvedValue({}),
  deleteFeed: jest.fn().mockResolvedValue({}),
  refreshFeed: jest.fn().mockResolvedValue({ ingested: 0 })
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => clientMocks)
}))
jest.mock('@/lib/gen/reading/v1/feeds_pb', () => ({ FeedService: {} }))

import useSWR from 'swr'
import {
  useFeeds,
  useCreateFeed,
  useUpdateFeed,
  useDeleteFeed,
  useRefreshFeed
} from '@/hooks/useBookFeeds'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

describe('useBookFeeds', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // @ts-expect-error -- partial SWRResponse is fine for these tests
    mockUseSWR.mockReturnValue({ data: undefined })
  })

  it('useFeeds queries the feed list key', async () => {
    renderHook(() => useFeeds())
    const [key, fetcher] = mockUseSWR.mock.calls[0] as [string, () => Promise<unknown>]
    expect(key).toBe(swrKeys.bookFeeds)
    await fetcher()
    expect(clientMocks.listFeeds).toHaveBeenCalledWith({})
  })

  it('useCreateFeed creates and invalidates feeds + library', async () => {
    const { result } = renderHook(() => useCreateFeed())
    const resp = await result.current('https://example.com/feed.xml', true)
    expect(clientMocks.createFeed).toHaveBeenCalledWith({
      url: 'https://example.com/feed.xml',
      koboSync: true
    })
    expect(resp.ingested).toBe(2)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.bookFeeds)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.books)
  })

  it('useUpdateFeed updates and invalidates feeds', async () => {
    const { result } = renderHook(() => useUpdateFeed())
    await result.current('f1', 'Title', false)
    expect(clientMocks.updateFeed).toHaveBeenCalledWith({
      feedId: 'f1',
      title: 'Title',
      koboSync: false
    })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.bookFeeds)
  })

  it('useDeleteFeed deletes and invalidates feeds', async () => {
    const { result } = renderHook(() => useDeleteFeed())
    await result.current('f1')
    expect(clientMocks.deleteFeed).toHaveBeenCalledWith({ feedId: 'f1' })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.bookFeeds)
  })

  it('useRefreshFeed skips the library invalidation when nothing was ingested', async () => {
    const { result } = renderHook(() => useRefreshFeed())
    await result.current('f1')
    expect(clientMocks.refreshFeed).toHaveBeenCalledWith({ feedId: 'f1' })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.bookFeeds)
    expect(mutateMock).not.toHaveBeenCalledWith(swrKeys.books)
  })
})
