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
  updateItem: jest.fn().mockResolvedValue({}),
  getFeedStats: jest.fn().mockResolvedValue({ stats: [], itemsPerDay: [] })
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
  useFetchFeedItemsPage,
  useCreateFeed,
  useDeleteFeed,
  useRefreshFeed,
  useUpdateItem,
  useFeedStats,
  useFeedsSummary
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

  it('useFeedItems queries the feed items key for the requested unread filter', async () => {
    renderHook(() => useFeedItems(true))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feedItems(true))
    await fetcher!()
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({ limit: 50, unreadOnly: true })
  })

  it('useFeedItems threads an optional feedId into the key and request', async () => {
    renderHook(() => useFeedItems(true, 'feed-1'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feedItems(true, 'feed-1'))
    await fetcher!()
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({
      limit: 50,
      unreadOnly: true,
      feedId: 'feed-1'
    })
  })

  it('useFetchFeedItemsPage fetches a page at the given offset', async () => {
    clientMocks.listFeedItems.mockResolvedValueOnce({ items: [{ id: 'i1' }], hasMore: true })
    const { result } = renderHook(() => useFetchFeedItemsPage(false))
    const page = await result.current(50)
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({
      limit: 50,
      offset: 50,
      unreadOnly: false
    })
    expect(page).toEqual({ items: [{ id: 'i1' }], hasMore: true })
  })

  it('useFetchFeedItemsPage threads an optional feedId through', async () => {
    clientMocks.listFeedItems.mockResolvedValueOnce({ items: [], hasMore: false })
    const { result } = renderHook(() => useFetchFeedItemsPage(false, 'feed-1'))
    await result.current(0)
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({
      limit: 50,
      offset: 0,
      unreadOnly: false,
      feedId: 'feed-1'
    })
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
    expect(mutateMock).toHaveBeenCalledWith(expect.any(Function))
  })

  it('useRefreshFeed skips the items invalidation when nothing was ingested', async () => {
    const { result } = renderHook(() => useRefreshFeed())
    await result.current('f1')
    expect(clientMocks.refreshFeed).toHaveBeenCalledWith({ feedId: 'f1' })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feeds)
    expect(mutateMock).toHaveBeenCalledTimes(1)
  })

  it('useRefreshFeed invalidates items when items were ingested', async () => {
    clientMocks.refreshFeed.mockResolvedValueOnce({ ingested: 3 })
    const { result } = renderHook(() => useRefreshFeed())
    await result.current('f1')
    expect(mutateMock).toHaveBeenCalledWith(expect.any(Function))
  })

  it('useUpdateItem partially updates an item and invalidates items', async () => {
    const { result } = renderHook(() => useUpdateItem())
    await result.current('item-1', { read: true })
    expect(clientMocks.updateItem).toHaveBeenCalledWith({ itemId: 'item-1', read: true })
    expect(mutateMock).toHaveBeenCalledWith(expect.any(Function))

    // The invalidation matcher must catch both unread-only and all-items
    // page keys, since a mutation doesn't know which variant is on-screen.
    const matcher = mutateMock.mock.calls[0]![0]
    if (typeof matcher !== 'function') throw new Error('expected a matcher function')
    expect(matcher(swrKeys.feedItems(true))).toBe(true)
    expect(matcher(swrKeys.feedItems(false))).toBe(true)
    expect(matcher(swrKeys.feeds)).toBe(false)
  })

  it('useUpdateItem passes readProgressPct through', async () => {
    const { result } = renderHook(() => useUpdateItem())
    await result.current('item-1', { readProgressPct: 42 })
    expect(clientMocks.updateItem).toHaveBeenCalledWith({
      itemId: 'item-1',
      readProgressPct: 42
    })
  })

  it('useFeedStats queries the feed stats key', async () => {
    renderHook(() => useFeedStats())
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feedStats)
    await fetcher!()
    expect(clientMocks.getFeedStats).toHaveBeenCalledWith({})
  })

  it('useFeedsSummary queries a small unread-items page and estimates an unread count from stats', async () => {
    mockUseSWR
      // @ts-expect-error -- partial SWRResponse is fine for these tests
      .mockReturnValueOnce({
        data: { items: [{ title: 'A', sourceUrl: 'u1', publishedAt: 'p1' }] }
      })
      // @ts-expect-error -- partial SWRResponse is fine for these tests
      .mockReturnValueOnce({
        data: {
          stats: [
            { itemCount: 10, readRate: 0.7 },
            { itemCount: 4, readRate: 0 }
          ]
        }
      })

    const { result } = renderHook(() => useFeedsSummary())

    const [itemsKey, itemsFetcher] = mockUseSWR.mock.calls[0]!
    expect(itemsKey).toBe(swrKeys.feedsSummary)
    await itemsFetcher!()
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({ limit: 5, unreadOnly: true })

    // unreadCount = round(10*(1-0.7)) + round(4*(1-0)) = 3 + 4
    expect(result.current.data).toEqual({
      unreadCount: 7,
      items: [{ title: 'A', sourceUrl: 'u1', publishedAt: 'p1' }]
    })
  })

  it('useFeedsSummary returns undefined data while items have not loaded', () => {
    // @ts-expect-error -- partial SWRResponse is fine for these tests
    mockUseSWR.mockReturnValueOnce({ data: undefined }).mockReturnValueOnce({ data: undefined })

    const { result } = renderHook(() => useFeedsSummary())

    expect(result.current.data).toBeUndefined()
  })
})
