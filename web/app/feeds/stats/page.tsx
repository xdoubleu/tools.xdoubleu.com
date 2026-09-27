import { Suspense } from 'react'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import FeedStatsClient from '@/components/feeds/FeedStatsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'

export default async function FeedStatsPage() {
  const feedsClient = await createServerClient(FeedService)
  const stats = await fetchOrNull(() => feedsClient.getFeedStats({}))

  return (
    <PageContainer>
      <SWRFallback fallback={stats ? { [swrKeys.feedStats]: stats } : {}}>
        <PageHeader
          title="Feed Stats"
          breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Stats' }]}
        />

        <Suspense fallback={<LoadingState />}>
          <FeedStatsClient />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
