import { Suspense } from 'react'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import FeedStatsClient from '@/components/feeds/FeedStatsClient'
import FeedRuleSuggestionsCard from '@/components/feeds/FeedRuleSuggestionsCard'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'

export default async function FeedStatsPage() {
  const feedsClient = await createServerClient(FeedService)
  const [stats, suggestions] = await Promise.all([
    fetchOrNull(() => feedsClient.getFeedStats({})),
    fetchOrNull(() => feedsClient.getFilterRuleSuggestions({}))
  ])
  const fallback = {
    ...(stats ? { [swrKeys.feedStats]: stats } : {}),
    ...(suggestions ? { [swrKeys.feedFilterRuleSuggestions]: suggestions } : {})
  }

  return (
    <PageContainer>
      <SWRFallback fallback={fallback}>
        <PageHeader
          title="Feed Stats"
          breadcrumb={[{ label: 'Feeds', href: '/feeds' }, { label: 'Stats' }]}
        />

        <div className="flex flex-col gap-6">
          <FeedRuleSuggestionsCard />
          <Suspense fallback={<LoadingState />}>
            <FeedStatsClient />
          </Suspense>
        </div>
      </SWRFallback>
    </PageContainer>
  )
}
