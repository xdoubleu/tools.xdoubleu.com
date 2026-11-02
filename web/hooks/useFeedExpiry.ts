import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { createServiceClient } from '@/lib/client'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import type { ListFeedItemsResponse, ListTopFeedItemsResponse } from '@/lib/gen/feeds/v1/feeds_pb'
import { noFocusRevalidate } from '@/hooks/useFeeds'

// useExpiredFeedItems lists the unread items that outlived their feed's
// lifetime, most recently expired first.
export function useExpiredFeedItems(feedId?: string) {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedItemsResponse, Error>(
    swrKeys.feedExpiredItems(feedId),
    () => client.listFeedItems({ limit: DEFAULT_PAGE_SIZE, expiredOnly: true, feedId }),
    noFocusRevalidate
  )
}

export function useFetchExpiredFeedItemsPage(feedId?: string) {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    (offset: number) =>
      client
        .listFeedItems({ limit: DEFAULT_PAGE_SIZE, offset, expiredOnly: true, feedId })
        .then((r) => ({ items: r.items, hasMore: r.hasMore })),
    [client, feedId]
  )
}

// useLastChanceFeedItems lists the unread items expiring within 48 hours,
// soonest first; `enabled` skips the request outside the unread inbox.
export function useLastChanceFeedItems(enabled: boolean) {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedItemsResponse, Error>(
    enabled ? swrKeys.feedLastChanceItems : null,
    () => client.listFeedItems({ limit: DEFAULT_PAGE_SIZE, lastChanceOnly: true }),
    noFocusRevalidate
  )
}

// useTopFeedItems fetches today's ranked list; `enabled` skips the request
// while the top view is off.
export function useTopFeedItems(enabled: boolean) {
  const client = createServiceClient(FeedService)
  return useSWR<ListTopFeedItemsResponse, Error>(
    enabled ? swrKeys.feedTopItems : null,
    () => client.listTopFeedItems({}),
    noFocusRevalidate
  )
}

// useUpdateFeedExpiry sets a feed's item lifetime in days (0 = never) and
// refetches the feeds and every list showing expiry times.
export function useUpdateFeedExpiry() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (feedId: string, title: string, expiryDays: number) => {
      await client.updateFeed({ feedId, title, expiryDays })
      await mutate(swrKeys.feeds)
      await mutate((key) => typeof key === 'string' && key.startsWith('/feeds/items'))
    },
    [client]
  )
}
