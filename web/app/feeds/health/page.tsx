import Link from 'next/link'
import { Suspense } from 'react'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { FeedService } from '@/lib/gen/feeds/v1/feeds_pb'
import UnhealthyFeeds from '@/components/feeds/UnhealthyFeeds'
import { PageContainer } from '@/components/ui/page-container'

// Admin-only RPC: non-admins get null and nothing renders.
export default async function FeedHealthPage() {
  const feedsClient = await createServerClient(FeedService)
  const unhealthyFeeds = await fetchOrNull(() => feedsClient.getUnhealthyFeeds({}))

  return (
    <PageContainer>
      <SWRFallback fallback={unhealthyFeeds ? { [swrKeys.unhealthyFeeds]: unhealthyFeeds } : {}}>
        <div className="mb-6 flex items-center justify-between gap-4">
          <h1 className="text-3xl font-bold">Feed Health</h1>
          <Link href="/feeds" className="text-sm text-accent underline-offset-4 hover:underline">
            Back to feeds
          </Link>
        </div>

        <Suspense fallback={<p className="text-muted">Loading…</p>}>
          <UnhealthyFeeds />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
