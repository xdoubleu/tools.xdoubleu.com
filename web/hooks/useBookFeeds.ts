import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { FeedService } from '@/lib/gen/reading/v1/feeds_pb'
import type { ListFeedsResponse, ListFeedItemsResponse } from '@/lib/gen/reading/v1/feeds_pb'

// RSS/Atom feed subscriptions for the reading library. Mutations invalidate
// the feed list; anything that can change library contents (ingest or
// delete items) also invalidates the library.

export function useFeeds() {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedsResponse, Error>(swrKeys.bookFeeds, () => client.listFeeds({}))
}

// useFeedItemBooks labels rss library items with the feed they came from, for
// the ad hoc feed-reader page (issue #476).
export function useFeedItemBooks() {
  const client = createServiceClient(FeedService)
  return useSWR<ListFeedItemsResponse, Error>(swrKeys.bookFeedItems, () => client.listFeedItems({}))
}

export function useCreateFeed() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (url: string, koboSync: boolean) => {
      const resp = await client.createFeed({ url, koboSync })
      await mutate(swrKeys.bookFeeds)
      // The initial import runs in the background (#430), so how many items
      // it will ingest is unknown here. Revalidate the library anyway — a
      // no-op refetch is cheap, and it picks up items that already landed.
      await mutate(swrKeys.books)
      return resp
    },
    [client]
  )
}

export function useUpdateFeed() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (feedId: string, title: string, koboSync: boolean) => {
      await client.updateFeed({ feedId, title, koboSync })
      await mutate(swrKeys.bookFeeds)
    },
    [client]
  )
}

export function useDeleteFeed() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (feedId: string) => {
      await client.deleteFeed({ feedId })
      await mutate(swrKeys.bookFeeds)
      // Deletion can remove books from the library (unless the user engaged
      // with them), so revalidate it the same way create/refresh do.
      await mutate(swrKeys.books)
    },
    [client]
  )
}

export function useRefreshFeed() {
  const client = useMemo(() => createServiceClient(FeedService), [])
  return useCallback(
    async (feedId: string) => {
      const resp = await client.refreshFeed({ feedId })
      await mutate(swrKeys.bookFeeds)
      if (resp.ingested > 0) await mutate(swrKeys.books)
      return resp
    },
    [client]
  )
}
