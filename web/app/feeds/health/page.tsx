import { Suspense } from 'react'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import UnhealthyFeeds from '@/components/feeds/UnhealthyFeeds'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'

// Admin-only RPC: non-admins get null and nothing renders.
export default async function FeedHealthPage() {
  const feedsClient = await createServerClient(FeedService)
  const unhealthyFeeds = await fetchOrNull(() => feedsClient.getUnhealthyFeeds({}))

  return (
    <PageContainer>
      <SWRFallback fallback={unhealthyFeeds ? { [swrKeys.unhealthyFeeds]: unhealthyFeeds } : {}}>
        <PageHeader
          title="Feed Health"
          breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Health' }]}
        />

        <Suspense fallback={<LoadingState />}>
          <UnhealthyFeeds />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
