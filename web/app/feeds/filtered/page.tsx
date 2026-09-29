import { Suspense } from 'react'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import FeedFilteredItemsClient from '@/components/feeds/FeedFilteredItemsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'

export default async function FeedFilteredPage({
  searchParams
}: {
  searchParams: Promise<{ feed?: string }>
}) {
  const { feed } = await searchParams
  const feedId = feed || undefined
  const feedsClient = await createServerClient(FeedService)
  const [filtered, feeds] = await Promise.all([
    fetchOrNull(() =>
      feedsClient.listFeedItems({ limit: DEFAULT_PAGE_SIZE, filteredOnly: true, feedId })
    ),
    fetchOrNull(() => feedsClient.listFeeds({}))
  ])

  return (
    <PageContainer>
      <SWRFallback
        fallback={{
          ...(filtered ? { [swrKeys.feedFilteredItems(feedId)]: filtered } : {}),
          ...(feeds ? { [swrKeys.feeds]: feeds } : {})
        }}
      >
        <PageHeader
          title="Filtered items"
          description="Items your filter rules hid. A restored item returns to the inbox as unread, and no rule hides it again."
          breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Filtered' }]}
        />

        <Suspense fallback={<LoadingState />}>
          <FeedFilteredItemsClient initialFeedId={feedId} />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
