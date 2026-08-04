import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { createServiceClient } from '@/lib/client'
import { FeedService, FeedKind } from '@/lib/gen/feeds/v1/feeds_pb'
import type {
  ListFeedsResponse,
  ListFeedItemsResponse,
  UpdateItemResponse,
  GetFeedStatsResponse
} from '@/lib/gen/feeds/v1/feeds_pb'

// RSS/Atom and email-newsletter feed subscriptions, standalone from the
// reading library (issue #734) — items are self-contained, so mutations
// only ever invalidate feeds-scoped keys.

// Items are paginated per unreadOnly variant (two independent SWR keys), so
// a mutation invalidates both rather than tracking which one is on-screen.
function mutateFeedItems() {
  return mutate((key) => typeof key === 'string' && key.startsWith('/feeds/items'))
}

export function useFeeds() {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedsResponse, Error>(swrKeys.feeds, () => client.listFeeds({}))
}

export function useFeedItems(unreadOnly: boolean, feedId?: string) {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedItemsResponse, Error>(swrKeys.feedItems(unreadOnly, feedId), () =>
    client.listFeedItems({ limit: DEFAULT_PAGE_SIZE, unreadOnly, feedId })
  )
}

export function useFetchFeedItemsPage(unreadOnly: boolean, feedId?: string) {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    (offset: number) =>
      client
        .listFeedItems({ limit: DEFAULT_PAGE_SIZE, offset, unreadOnly, feedId })
        .then((r) => ({ items: r.items, hasMore: r.hasMore })),
    [client, unreadOnly, feedId]
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
  favourite?: boolean
  readProgressPct?: number
}

// useUpdateItem partially updates an item's read/dismissed/favourite/
// read-progress state (proto3 field-presence, so unset keys are left
// unchanged server-side).
export function useUpdateItem() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (itemId: string, updates: UpdateItemInput): Promise<UpdateItemResponse> => {
      const resp = await client.updateItem({ itemId, ...updates })
      await mutateFeedItems()
      return resp
    },
    [client]
  )
}

// useFeedStats fetches issue #798's per-feed posting-cadence/read-completion
// stats plus the trailing-90-day items-per-day histogram.
export function useFeedStats() {
  const client = createServiceClient(FeedService)
  return useSWR<GetFeedStatsResponse, Error>(swrKeys.feedStats, () => client.getFeedStats({}))
}
