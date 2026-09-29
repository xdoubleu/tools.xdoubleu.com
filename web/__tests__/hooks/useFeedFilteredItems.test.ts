import { renderHook } from '@testing-library/react'

const mutateMock = jest.fn()
jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  mutate: (...args: unknown[]) => mutateMock(...args)
}))

const clientMocks = {
  listFeedItems: jest.fn().mockResolvedValue({ items: [{ id: 'i1' }], hasMore: true }),
  restoreFeedItem: jest.fn().mockResolvedValue({ item: { id: 'i1' } })
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

  it('useRestoreFeedItem restores and refreshes items, stats, summary and rule counts', async () => {
    const { result } = renderHook(() => useRestoreFeedItem())
    const resp = await result.current('i1')
    expect(resp.item?.id).toBe('i1')
    expect(clientMocks.restoreFeedItem).toHaveBeenCalledWith({ itemId: 'i1' })
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedStats)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedsSummary)
    expect(mutateMock).toHaveBeenCalledWith(swrKeys.feedFilterRules)
    const itemsMatcher = mutateMock.mock.calls.find(([k]) => typeof k === 'function')![0]
    expect(itemsMatcher(swrKeys.feedItems(true))).toBe(true)
    expect(itemsMatcher(swrKeys.feedFilteredItems())).toBe(true)
  })
})
