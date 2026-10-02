import { useCallback, useEffect, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { createServiceClient } from '@/lib/client'
import { FeedService, FeedKind } from '@/lib/gen/feeds/v1/feeds_pb'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import { deleteFeedWrite, deleteFilterRuleWrite, updateItemWrite } from '@/lib/feeds/offlineWrites'
import { prefetchFeedBodies } from '@/lib/feeds/prefetch'
import type {
  CreateFilterRuleResponse,
  FilterRuleKind,
  ListFilterRulesResponse,
  ListFeedsResponse,
  ListFeedItemsResponse,
  GetFeedItemResponse,
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

// Invalidates every cached item list, filtered ones included. Only for
// create and refresh; other changes go through the offline outbox.
function mutateFeedItems() {
  return mutate((key) => typeof key === 'string' && key.startsWith('/feeds/items'))
}

// No refetch on focus/reconnect: pages are server-prefetched and refetching
// wastes egress.
export const noAutoRevalidate = {
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

// usePrefetchFeedBodies saves unread article bodies for offline reading.
export function usePrefetchFeedBodies() {
  useEffect(() => {
    void prefetchFeedBodies(createServiceClient(FeedService))
  }, [])
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

// Writes below go through the offline outbox and show at once; a rejection
// throws (or, for item state, shows in the offline banner) and rolls back.

export function useDeleteFeed() {
  return useCallback(async (feedId: string) => {
    await sendWrite(enqueueWrite(deleteFeedWrite, { feedId }))
  }, [])
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
  return useCallback(async (itemId: string, updates: UpdateItemInput) => {
    await enqueueWrite(updateItemWrite, { itemId, ...updates }, new Date().toISOString())
  }, [])
}

// useFilterRules lists the caller's filter rules with their filtered counts.
export function useFilterRules() {
  const client = createServiceClient(FeedService)
  return useSWR<ListFilterRulesResponse, Error>(
    swrKeys.feedFilterRules,
    () => client.listFilterRules({}),
    noAutoRevalidate
  )
}

export interface CreateFilterRuleInput {
  // Empty applies the rule to all feeds.
  feedId: string
  kind: FilterRuleKind
  value: string
}

// useCreateFilterRule creates a rule and refetches the rules and the
// suggestions it may cover; when it filtered existing items, the item lists
// and stats that counted them are refetched too.
export function useCreateFilterRule() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (input: CreateFilterRuleInput): Promise<CreateFilterRuleResponse> => {
      const resp = await client.createFilterRule(input)
      await mutate(swrKeys.feedFilterRules)
      await mutate(swrKeys.feedFilterRuleSuggestions)
      if ((resp.rule?.filteredCount ?? 0) > 0) {
        await mutateFeedItems()
        await mutate(swrKeys.feedStats)
        await mutate(swrKeys.feedsSummary)
      }
      return resp
    },
    [client]
  )
}

// useDeleteFilterRule deletes a rule; its items stay filtered, and the
// suggestions it covered can return.
export function useDeleteFilterRule() {
  return useCallback(async (ruleId: string) => {
    await sendWrite(enqueueWrite(deleteFilterRuleWrite, { ruleId }))
  }, [])
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
