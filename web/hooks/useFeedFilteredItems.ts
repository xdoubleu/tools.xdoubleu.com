import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { createServiceClient } from '@/lib/client'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import type { ListFeedItemsResponse, RestoreFeedItemResponse } from '@/lib/gen/feeds/v1/feeds_pb'
import { mutateFeedItems, noAutoRevalidate } from '@/hooks/useFeeds'

// useFilteredFeedItems lists the items filter rules hide, newest filtered
// first, each with the rule that matched.
export function useFilteredFeedItems(feedId?: string) {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedItemsResponse, Error>(
    swrKeys.feedFilteredItems(feedId),
    () => client.listFeedItems({ limit: DEFAULT_PAGE_SIZE, filteredOnly: true, feedId }),
    noAutoRevalidate
  )
}

export function useFetchFilteredFeedItemsPage(feedId?: string) {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    (offset: number) =>
      client
        .listFeedItems({ limit: DEFAULT_PAGE_SIZE, offset, filteredOnly: true, feedId })
        .then((r) => ({ items: r.items, hasMore: r.hasMore })),
    [client, feedId]
  )
}

// useRestoreFeedItem returns a filtered item to the inbox, then refreshes
// everything that counts it: item lists, stats, the summary and rule counts.
export function useRestoreFeedItem() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (itemId: string): Promise<RestoreFeedItemResponse> => {
      const resp = await client.restoreFeedItem({ itemId })
      await mutateFeedItems()
      await mutate(swrKeys.feedStats)
      await mutate(swrKeys.feedsSummary)
      await mutate(swrKeys.feedFilterRules)
      return resp
    },
    [client]
  )
}
