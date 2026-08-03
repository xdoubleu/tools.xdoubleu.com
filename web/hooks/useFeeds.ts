import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { FeedService, FeedKind } from '@/lib/gen/feeds/v1/feeds_pb'
import type {
  ListFeedsResponse,
  ListFeedItemsResponse,
  UpdateItemResponse
} from '@/lib/gen/feeds/v1/feeds_pb'

// RSS/Atom and email-newsletter feed subscriptions, standalone from the
// reading library (issue #734) — items are self-contained, so mutations
// only ever invalidate feeds-scoped keys.

export function useFeeds() {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedsResponse, Error>(swrKeys.feeds, () => client.listFeeds({}))
}

export function useFeedItems() {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedItemsResponse, Error>(swrKeys.feedItems, () => client.listFeedItems({}))
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
      await mutate(swrKeys.feedItems)
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
      if (resp.ingested > 0) await mutate(swrKeys.feedItems)
      return resp
    },
    [client]
  )
}

export interface UpdateItemInput {
  read?: boolean
  dismissed?: boolean
  favourite?: boolean
}

// useUpdateItem partially updates an item's read/dismissed/favourite state
// (proto3 field-presence, so unset keys are left unchanged server-side).
export function useUpdateItem() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (itemId: string, updates: UpdateItemInput): Promise<UpdateItemResponse> => {
      const resp = await client.updateItem({ itemId, ...updates })
      await mutate(swrKeys.feedItems)
      return resp
    },
    [client]
  )
}
