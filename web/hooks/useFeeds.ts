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
  GetFeedStatsResponse
} from '@/lib/gen/feeds/v1/feeds_pb'

// FEEDS_SUMMARY_ITEM_LIMIT bounds the reading dashboard's feeds widget.
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

// RSS/Atom and email-newsletter feed subscriptions, standalone from the
// reading library (issue #734) — items are self-contained, so mutations
// only ever invalidate feeds-scoped keys.

// Items are paginated per unreadOnly variant (two independent SWR keys), so
// a mutation invalidates both rather than tracking which one is on-screen.
// Only for mutations that change which items exist (create/delete/refresh) —
// per-item state changes use patchCachedItem instead, which avoids refetching
// a whole page.
function mutateFeedItems() {
  return mutate((key) => typeof key === 'string' && key.startsWith('/feeds/items'))
}

// patchCachedItem writes an updated item into every cached list page in
// place, with revalidate:false. UpdateItem already returns the new row, so
// refetching the list to learn what we were just told is pure waste — and it
// used to fire on every debounced scroll tick, re-pulling a page of items
// each time (issue #1027).
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

// noAutoRevalidate stops SWR re-running these fetches on every tab focus and
// reconnect, matching useGames/useAuth. Feed pages are server-prefetched into
// the SWR cache already, so the default behaviour meant re-pulling a whole
// page of items for no new information — a large share of the egress that
// exhausted the Supabase quota in issue #1027.
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

export function useFeedItems(unreadOnly: boolean, feedId?: string, bookmarkedOnly?: boolean) {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedItemsResponse, Error>(
    swrKeys.feedItems(unreadOnly, feedId, bookmarkedOnly),
    () => client.listFeedItems({ limit: DEFAULT_PAGE_SIZE, unreadOnly, feedId, bookmarkedOnly }),
    noAutoRevalidate
  )
}

// useFeedItem fetches one item's article body. List responses carry only a
// hasContent flag (issue #1027), so the reader pulls the body when it opens
// an article and SWR keeps it cached for the rest of the session.
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

// useUpdateItem partially updates an item's read/dismissed/bookmarked/
// read-progress state (proto3 field-presence, so unset keys are left
// unchanged server-side).
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

// useFeedStats fetches issue #798's per-feed posting-cadence/read-completion
// stats plus the trailing-90-day items-per-day histogram.
export function useFeedStats() {
  const client = createServiceClient(FeedService)
  return useSWR<GetFeedStatsResponse, Error>(
    swrKeys.feedStats,
    () => client.getFeedStats({}),
    noAutoRevalidate
  )
}

// useFeedsSummary backs the reading dashboard's feeds widget (issue #737):
// a handful of recent unread items plus an unread count. There's no
// dedicated summary RPC for the owner's own authenticated view — unlike the
// public dashboard's GetSharedFeedsSummary, this composes two existing
// FeedService calls instead. The unread count is a best-effort estimate
// (per-feed item_count * (1 - read_rate), rounded and summed) rather than an
// exact count, which is an acceptable tradeoff for a summary widget.
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
