import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))

const clientMocks = {
  listFeedItems: jest.fn().mockResolvedValue({ items: [{ id: 'i1' }], hasMore: true })
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => clientMocks)
}))
jest.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: jest.fn(async () => ({ status: () => 'sent' })),
  sendWrite: jest.fn(async (pending: Promise<unknown>) => {
    await pending
    return 'sent'
  })
}))
jest.mock('@/lib/feeds/offlineWrites', () => ({ restoreFeedItemWrite: { id: 'restore' } }))
jest.mock('@/lib/feeds/prefetch', () => ({ prefetchFeedBodies: jest.fn() }))
jest.mock('@/lib/gen/feeds/v1/feeds_pb', () => ({
  FeedService: {},
  FeedKind: { UNSPECIFIED: 0, RSS: 1, EMAIL: 2 }
}))

import useSWR from 'swr'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import { restoreFeedItemWrite } from '@/lib/feeds/offlineWrites'
import {
  useFetchFilteredFeedItemsPage,
  useFilteredFeedItems,
  useRestoreFeedItem
} from '@/hooks/useFeedFilteredItems'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

describe('filtered item hooks', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // @ts-expect-error -- partial SWRResponse is fine for these tests
    mockUseSWR.mockReturnValue({ data: undefined })
  })

  it('useFilteredFeedItems lists filtered items per feed without focus revalidation', async () => {
    renderHook(() => useFilteredFeedItems('feed-1'))
    const [key, fetcher, options] = mockUseSWR.mock.calls[0]!
    expect(key).toBe(swrKeys.feedFilteredItems('feed-1'))
    expect(options).toMatchObject({ revalidateOnFocus: false })
    await fetcher!()
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({
      limit: DEFAULT_PAGE_SIZE,
      filteredOnly: true,
      feedId: 'feed-1'
    })
  })

  it('keys filtered items under the item-list prefix so rule changes refresh them', () => {
    expect(swrKeys.feedFilteredItems().startsWith('/feeds/items')).toBe(true)
    expect(swrKeys.feedFilteredItems('a')).not.toBe(swrKeys.feedFilteredItems('b'))
  })

  it('useFetchFilteredFeedItemsPage fetches the next page at an offset', async () => {
    const { result } = renderHook(() => useFetchFilteredFeedItemsPage(undefined))
    await expect(result.current(20)).resolves.toEqual({ items: [{ id: 'i1' }], hasMore: true })
    expect(clientMocks.listFeedItems).toHaveBeenCalledWith({
      limit: DEFAULT_PAGE_SIZE,
      offset: 20,
      filteredOnly: true,
      feedId: undefined
    })
  })

  it('useFetchFilteredFeedItemsPage follows a feed change', async () => {
    const { result, rerender } = renderHook(({ feedId }) => useFetchFilteredFeedItemsPage(feedId), {
      initialProps: { feedId: 'a' }
    })
    rerender({ feedId: 'b' })
    await result.current(0)
    expect(clientMocks.listFeedItems).toHaveBeenLastCalledWith(
      expect.objectContaining({ feedId: 'b' })
    )
  })

  it('useRestoreFeedItem sends the restore through the outbox', async () => {
    const { result } = renderHook(() => useRestoreFeedItem())
    await result.current('i1')
    expect(enqueueWrite).toHaveBeenCalledWith(restoreFeedItemWrite, { itemId: 'i1' })
    expect(sendWrite).toHaveBeenCalled()
  })
})
