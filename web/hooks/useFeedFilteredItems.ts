import { useCallback, useMemo } from 'react'
import useSWR from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { createServiceClient } from '@/lib/client'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import type { ListFeedItemsResponse } from '@/lib/gen/feeds/v1/feeds_pb'
import { noAutoRevalidate } from '@/hooks/useFeeds'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import { restoreFeedItemWrite } from '@/lib/feeds/offlineWrites'

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

// useRestoreFeedItem returns a filtered item to the inbox through the offline
// outbox; once sent, everything that counts it is refetched.
export function useRestoreFeedItem() {
  return useCallback(async (itemId: string) => {
    await sendWrite(enqueueWrite(restoreFeedItemWrite, { itemId }))
  }, [])
}
