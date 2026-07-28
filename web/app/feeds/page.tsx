import { Suspense } from 'react'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/reading/v1/library_pb'
import { FeedService } from '@/lib/gen/reading/v1/feeds_pb'
import FeedReaderClient from '@/components/reading/FeedReaderClient'
import FeedsHeader from '@/components/reading/FeedsHeader'
import { PageContainer } from '@/components/ui/page-container'

export default async function FeedsPage() {
  const libraryClient = await createServerClient(LibraryService)
  const feedsClient = await createServerClient(FeedService)

  const [library, feedItems, feeds] = await Promise.all([
    fetchOrNull(() => libraryClient.getLibrary({})),
    fetchOrNull(() => feedsClient.listFeedItems({})),
    fetchOrNull(() => feedsClient.listFeeds({}))
  ])

  return (
    <PageContainer className="p-6">
      <SWRFallback
        fallback={{
          ...(library ? { [swrKeys.books]: library } : {}),
          ...(feedItems ? { [swrKeys.bookFeedItems]: feedItems } : {}),
          ...(feeds ? { [swrKeys.bookFeeds]: feeds } : {})
        }}
      >
        <FeedsHeader />

        <Suspense fallback={<p className="text-muted">Loading…</p>}>
          <FeedReaderClient />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
