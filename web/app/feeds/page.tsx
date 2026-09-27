import { Suspense } from 'react'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import FeedReaderClient from '@/components/feeds/FeedReaderClient'
import FeedsHeader from '@/components/feeds/FeedsHeader'
import { PageContainer } from '@/components/ui/page-container'
import { LoadingState } from '@/components/ui/states'

export default async function FeedsPage() {
  const feedsClient = await createServerClient(FeedService)

  const [feedItems, feeds] = await Promise.all([
    fetchOrNull(() => feedsClient.listFeedItems({ unreadOnly: true })),
    fetchOrNull(() => feedsClient.listFeeds({}))
  ])

  return (
    <PageContainer>
      <SWRFallback
        fallback={{
          ...(feedItems ? { [swrKeys.feedItems(true)]: feedItems } : {}),
          ...(feeds ? { [swrKeys.feeds]: feeds } : {})
        }}
      >
        <FeedsHeader />

        <Suspense fallback={<LoadingState />}>
          <FeedReaderClient />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
