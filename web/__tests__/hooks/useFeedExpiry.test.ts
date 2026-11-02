import { renderHook } from '@testing-library/react'

vi.mock('swr', () => ({ __esModule: true, default: vi.fn(), mutate: vi.fn() }))

const clientMocks = {
  listFeedItems: vi.fn().mockResolvedValue({ items: [{ id: 'i1' }], hasMore: true }),
  listTopFeedItems: vi.fn().mockResolvedValue({ items: [] }),
  updateFeed: vi.fn().mockResolvedValue({})
}
vi.mock('@/lib/client', () => ({ createServiceClient: vi.fn(() => clientMocks) }))
vi.mock('@/lib/offline/outbox', () => ({ enqueueWrite: vi.fn(), sendWrite: vi.fn() }))
vi.mock('@/lib/feeds/offlineWrites', () => ({}))
vi.mock('@/lib/feeds/prefetch', () => ({ prefetchFeedBodies: vi.fn() }))
vi.mock('@/lib/gen/feeds/v1/feeds_pb', () => ({ FeedService: {}, FeedKind: { RSS: 1 } }))

import useSWR, { mutate } from 'swr'
import {
  useExpiredFeedItems,
  useFetchExpiredFeedItemsPage,
  useLastChanceFeedItems,
  useTopFeedItems,
  useUpdateFeedExpiry
} from '@/hooks/useFeedExpiry'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = vi.mocked(useSWR)

describe('feed expiry hooks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // @ts-expect-error -- partial SWRResponse is fine for these tests
    mockUseSWR.mockReturnValue({ data: undefined })
  })

  it('useExpiredFeedItems lists expired items per feed', async () => {
    renderHook(() => useExpiredFeedItems('feed-1'))
    const [key, fetcher, options] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feedExpiredItems('feed-1'))
    expect(options).toMatchObject({ revalidateOnFocus: false })
    await fetcher!()
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({
      limit: DEFAULT_PAGE_SIZE,
      expiredOnly: true,
      feedId: 'feed-1'
    })
  })

  it('keys lists under the item-list prefix so changes refresh them', () => {
    for (const key of [
      swrKeys.feedExpiredItems(),
      swrKeys.feedLastChanceItems,
      swrKeys.feedTopItems
    ]) {
      expect(key.startsWith('/feeds/items')).toBe(true)
    }
  })

  it('useFetchExpiredFeedItemsPage fetches the next page at an offset', async () => {
    const { result } = renderHook(() => useFetchExpiredFeedItemsPage(undefined))
    await expect(result.current(20)).resolves.toEqual({ items: [{ id: 'i1' }], hasMore: true })
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({
      limit: DEFAULT_PAGE_SIZE,
      offset: 20,
      expiredOnly: true,
      feedId: undefined
    })
  })

  it('useLastChanceFeedItems only fetches when enabled', async () => {
    renderHook(() => useLastChanceFeedItems(false))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()

    renderHook(() => useLastChanceFeedItems(true))
    const [key, fetcher] = mockUseSWR.mock.calls[1]!
    expect(key).toBe(swrKeys.feedLastChanceItems)
    await fetcher!()
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({
      limit: DEFAULT_PAGE_SIZE,
      lastChanceOnly: true
    })
  })

  it('useTopFeedItems only fetches when enabled', async () => {
    renderHook(() => useTopFeedItems(false))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()

    renderHook(() => useTopFeedItems(true))
    const [key, fetcher] = mockUseSWR.mock.calls[1]!
    expect(key).toBe(swrKeys.feedTopItems)
    await fetcher!()
    expect(clientMocks.listTopFeedItems).toHaveBeenCalledWith({})
  })

  it('useUpdateFeedExpiry saves the lifetime then refetches feeds and item lists', async () => {
    const { result } = renderHook(() => useUpdateFeedExpiry())
    await result.current('feed-1', 'Blog', 3)

    expect(clientMocks.updateFeed).toHaveBeenCalledWith({
      feedId: 'feed-1',
      title: 'Blog',
      expiryDays: 3
    })
    expect(mutate).toHaveBeenCalledWith(swrKeys.feeds)
    const sweep = vi.mocked(mutate).mock.calls[1]![0]
    expect(typeof sweep).toBe('function')
    if (typeof sweep !== 'function') return
    expect(sweep('/feeds/items/last-chance')).toBe(true)
    expect(sweep('/feeds/item/abc')).toBe(false)
  })
})
