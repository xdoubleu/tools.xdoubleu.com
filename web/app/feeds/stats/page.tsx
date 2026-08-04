import Link from 'next/link'
import { Suspense } from 'react'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import FeedStatsClient from '@/components/feeds/FeedStatsClient'
import { PageContainer } from '@/components/ui/page-container'

export default async function FeedStatsPage() {
  const feedsClient = await createServerClient(FeedService)
  const stats = await fetchOrNull(() => feedsClient.getFeedStats({}))

  return (
    <PageContainer className="p-6">
      <SWRFallback fallback={stats ? { [swrKeys.feedStats]: stats } : {}}>
        <div className="mb-6 flex items-center justify-between gap-4">
          <h1 className="text-3xl font-bold">Feed Stats</h1>
          <Link href="/feeds" className="text-sm text-accent underline-offset-4 hover:underline">
            Back to feeds
          </Link>
        </div>

        <Suspense fallback={<p className="text-muted">Loading…</p>}>
          <FeedStatsClient />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
