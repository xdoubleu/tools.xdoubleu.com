import type { Client } from '@connectrpc/connect'
import { loadEntry, saveEntry } from '@/lib/offline/store'
import { swrKeys } from '@/lib/swrKeys'
import type { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'

const PREFETCH_LIMIT = 100

let started = false

/**
 * Saves the bodies of up to 100 unread articles for offline reading, once per
 * page load and skipping ones already saved. Text only: images aren't cached.
 */
export async function prefetchFeedBodies(client: Client<typeof FeedService>): Promise<void> {
  if (started || !navigator.onLine) return
  started = true
  try {
    const { items } = await client.listFeedItems({ limit: PREFETCH_LIMIT, unreadOnly: true })
    for (const item of items) {
      const key = swrKeys.feedItem(item.id)
      if (!item.hasContent || (await loadEntry(key))) continue
      await saveEntry(key, await client.getFeedItem({ itemId: item.id }))
    }
  } catch {
    // Best effort: a later visit retries what's missing.
    started = false
  }
}
