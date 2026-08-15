import type { Metadata } from 'next'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicReadingDashboardService } from '@/lib/gen/dashboard/v1/reading_pb'
import ReadingDashboardPublicClient from '@/components/dashboard/ReadingDashboardPublicClient'
import { PageContainer } from '@/components/ui/page-container'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared reading dashboard',
  robots: { index: false, follow: false }
}

export default async function ReadingDashboardPublicPage({
  params
}: {
  params: Promise<{ token: string }>
}) {
  const { token } = await params
  const client = await createServerClient(PublicReadingDashboardService)
  const [library, feedsSummary] = await Promise.all([
    fetchOrNull(() => client.getSharedLibrary({ token })),
    fetchOrNull(() => client.getSharedFeedsSummary({ token }))
  ])

  return (
    <PageContainer className="p-6 lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden lg:p-4">
      <h1 className="mb-6 text-3xl font-bold lg:mb-3 lg:text-2xl">
        {library?.displayName ? `${library.displayName}'s reading` : 'Shared reading'}
      </h1>
      <SWRFallback
        fallback={{
          ...(library ? { [swrKeys.dashboardReading(token)]: library } : {}),
          ...(feedsSummary ? { [swrKeys.dashboardFeedsSummary(token)]: feedsSummary } : {})
        }}
      >
        <ReadingDashboardPublicClient token={token} initialData={library ?? undefined} />
      </SWRFallback>
    </PageContainer>
  )
}
