import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { createServiceClient } from '@/lib/client'
import { FeedService, FeedKind } from '@/lib/gen/feeds/v1/feeds_pb'
import type {
  Item,
  ListFeedsResponse,
  ListFeedItemsResponse,
  GetFeedItemResponse,
  UpdateItemResponse,
  GetFeedStatsResponse,
  GetUnhealthyFeedsResponse
} from '@/lib/gen/feeds/v1/feeds_pb'

const FEEDS_SUMMARY_ITEM_LIMIT = 5

interface FeedsSummaryItem {
  title: string
  sourceUrl: string
  publishedAt: string
}

export interface FeedsSummary {
  unreadCount: number
  items: FeedsSummaryItem[]
}

// Invalidates both unreadOnly variants. Only for create/delete/refresh;
// per-item changes use patchCachedItem.
function mutateFeedItems() {
  return mutate((key) => typeof key === 'string' && key.startsWith('/feeds/items'))
}

// patchCachedItem writes UpdateItem's returned row into every cached page
// without refetching.
function patchCachedItem(updated: Item) {
  return mutate(
    (key) => typeof key === 'string' && key.startsWith('/feeds/items'),
    (current?: ListFeedItemsResponse) => {
      if (!current?.items.some((item) => item.id === updated.id)) return current
      return {
        ...current,
        items: current.items.map((item) => (item.id === updated.id ? updated : item))
      } as ListFeedItemsResponse
    },
    { revalidate: false }
  )
}

// No refetch on focus/reconnect: pages are server-prefetched and refetching
// wastes egress.
const noAutoRevalidate = {
  revalidateOnFocus: false,
  revalidateOnReconnect: false
} as const

export function useFeeds() {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedsResponse, Error>(
    swrKeys.feeds,
    () => client.listFeeds({}),
    noAutoRevalidate
  )
}

// useUnhealthyFeeds lists every user's failing feeds (admin-only); enabled
// keeps non-admins from sending a request that would be denied.
export function useUnhealthyFeeds(enabled: boolean) {
  const client = createServiceClient(FeedService)
  return useSWR<GetUnhealthyFeedsResponse, Error>(enabled ? swrKeys.unhealthyFeeds : null, () =>
    client.getUnhealthyFeeds({})
  )
}

export function useFeedItems(unreadOnly: boolean, feedId?: string, bookmarkedOnly?: boolean) {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedItemsResponse, Error>(
    swrKeys.feedItems(unreadOnly, feedId, bookmarkedOnly),
    () => client.listFeedItems({ limit: DEFAULT_PAGE_SIZE, unreadOnly, feedId, bookmarkedOnly }),
    noAutoRevalidate
  )
}

// useFeedItem fetches one article body; list responses only carry hasContent.
export function useFeedItem(itemId: string | null) {
  const client = createServiceClient(FeedService)
  return useSWR<GetFeedItemResponse, Error>(
    itemId ? swrKeys.feedItem(itemId) : null,
    itemId ? () => client.getFeedItem({ itemId }) : null,
    noAutoRevalidate
  )
}

export function useFetchFeedItemsPage(
  unreadOnly: boolean,
  feedId?: string,
  bookmarkedOnly?: boolean
) {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    (offset: number) =>
      client
        .listFeedItems({ limit: DEFAULT_PAGE_SIZE, offset, unreadOnly, feedId, bookmarkedOnly })
        .then((r) => ({ items: r.items, hasMore: r.hasMore })),
    [client, unreadOnly, feedId, bookmarkedOnly]
  )
}

export function useCreateFeed() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (url: string, kind: FeedKind = FeedKind.RSS, title = '') => {
      const resp = await client.createFeed({ url, kind, title })
      await mutate(swrKeys.feeds)
      return resp
    },
    [client]
  )
}

export function useDeleteFeed() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (feedId: string) => {
      await client.deleteFeed({ feedId })
      await mutate(swrKeys.feeds)
      await mutateFeedItems()
    },
    [client]
  )
}

export function useRefreshFeed() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (feedId: string) => {
      const resp = await client.refreshFeed({ feedId })
      await mutate(swrKeys.feeds)
      if (resp.ingested > 0) await mutateFeedItems()
      return resp
    },
    [client]
  )
}

export interface UpdateItemInput {
  read?: boolean
  dismissed?: boolean
  bookmarked?: boolean
  readProgressPct?: number
}

// useUpdateItem partially updates an item; unset keys are left unchanged.
export function useUpdateItem() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (itemId: string, updates: UpdateItemInput): Promise<UpdateItemResponse> => {
      const resp = await client.updateItem({ itemId, ...updates })
      if (resp.item) await patchCachedItem(resp.item)
      return resp
    },
    [client]
  )
}

// useFeedStats fetches per-feed cadence/read stats and the 90-day histogram.
export function useFeedStats() {
  const client = createServiceClient(FeedService)
  return useSWR<GetFeedStatsResponse, Error>(
    swrKeys.feedStats,
    () => client.getFeedStats({}),
    noAutoRevalidate
  )
}

// useFeedsSummary backs the reading dashboard's feeds widget. The unread count
// is an estimate: sum of item_count * (1 - read_rate).
export function useFeedsSummary() {
  const itemsClient = createServiceClient(FeedService)
  const {
    data: itemsData,
    error,
    isLoading
  } = useSWR<ListFeedItemsResponse, Error>(
    swrKeys.feedsSummary,
    () => itemsClient.listFeedItems({ limit: FEEDS_SUMMARY_ITEM_LIMIT, unreadOnly: true }),
    noAutoRevalidate
  )
  const { data: statsData } = useFeedStats()

  const unreadCount =
    statsData?.stats.reduce(
      (sum, stats) => sum + Math.round(stats.itemCount * (1 - stats.readRate)),
      0
    ) ?? 0

  const data: FeedsSummary | undefined = itemsData
    ? {
        unreadCount,
        items: itemsData.items.map((item) => ({
          title: item.title,
          sourceUrl: item.sourceUrl,
          publishedAt: item.publishedAt
        }))
      }
    : undefined

  return { data, error, isLoading }
}
