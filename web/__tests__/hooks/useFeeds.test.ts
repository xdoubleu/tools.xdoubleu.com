import { renderHook } from '@testing-library/react'

const mutateMock = jest.fn()
jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  mutate: (...args: unknown[]) => mutateMock(...args)
}))

const clientMocks = {
  listFeeds: jest.fn().mockResolvedValue({ feeds: [] }),
  listFeedItems: jest.fn().mockResolvedValue({ items: [] }),
  createFeed: jest.fn().mockResolvedValue({}),
  deleteFeed: jest.fn().mockResolvedValue({}),
  refreshFeed: jest.fn().mockResolvedValue({ ingested: 0 }),
  updateItem: jest.fn().mockResolvedValue({})
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => clientMocks)
}))
jest.mock('@/lib/gen/feeds/v1/feeds_pb', () => ({
  FeedService: {},
  FeedKind: { UNSPECIFIED: 0, RSS: 1, EMAIL: 2 }
}))

import useSWR from 'swr'
import {
  useFeeds,
  useFeedItems,
  useCreateFeed,
  useDeleteFeed,
  useRefreshFeed,
  useUpdateItem
} from '@/hooks/useFeeds'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

describe('useFeeds', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // @ts-expect-error -- partial SWRResponse is fine for these tests
    mockUseSWR.mockReturnValue({ data: undefined })
  })

  it('useFeeds queries the feed list key', async () => {
    renderHook(() => useFeeds())
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feeds)
    await fetcher!()
    expect(clientMocks.listFeeds).toHaveBeenCalledWith({})
  })

  it('useFeedItems queries the feed items key', async () => {
    renderHook(() => useFeedItems())
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feedItems)
    await fetcher!()
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({})
  })

  it('useCreateFeed creates and invalidates feeds', async () => {
    const { result } = renderHook(() => useCreateFeed())
    await result.current('https://example.com/feed.xml')
    expect(clientMocks.createFeed).toHaveBeenCalledWith({
      url: 'https://example.com/feed.xml',
      kind: 1,
      title: ''
    })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feeds)
  })

  it('useCreateFeed passes through an explicit kind (email feeds)', async () => {
    const { result } = renderHook(() => useCreateFeed())
    await result.current('', 2)
    expect(clientMocks.createFeed).toHaveBeenCalledWith({
      url: '',
      kind: 2,
      title: ''
    })
  })

  it('useCreateFeed passes through an optional title (email newsletter name)', async () => {
    const { result } = renderHook(() => useCreateFeed())
    await result.current('', 2, 'My Substack')
    expect(clientMocks.createFeed).toHaveBeenCalledWith({
      url: '',
      kind: 2,
      title: 'My Substack'
    })
  })

  it('useDeleteFeed deletes and invalidates feeds + items', async () => {
    const { result } = renderHook(() => useDeleteFeed())
    await result.current('f1')
    expect(clientMocks.deleteFeed).toHaveBeenCalledWith({ feedId: 'f1' })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feeds)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedItems)
  })

  it('useRefreshFeed skips the items invalidation when nothing was ingested', async () => {
    const { result } = renderHook(() => useRefreshFeed())
    await result.current('f1')
    expect(clientMocks.refreshFeed).toHaveBeenCalledWith({ feedId: 'f1' })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feeds)
    expect(mutateMock).not.toHaveBeenCalledWith(swrKeys.feedItems)
  })

  it('useRefreshFeed invalidates items when items were ingested', async () => {
    clientMocks.refreshFeed.mockResolvedValueOnce({ ingested: 3 })
    const { result } = renderHook(() => useRefreshFeed())
    await result.current('f1')
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedItems)
  })

  it('useUpdateItem partially updates an item and invalidates items', async () => {
    const { result } = renderHook(() => useUpdateItem())
    await result.current('item-1', { read: true })
    expect(clientMocks.updateItem).toHaveBeenCalledWith({ itemId: 'item-1', read: true })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedItems)
  })
})
