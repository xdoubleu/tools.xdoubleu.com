import type { Metadata } from 'next'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicReadingDashboardService } from '@/lib/gen/dashboard/v1/reading_pb'
import ReadingDashboardPublicLibrary from '@/components/dashboard/ReadingDashboardPublicLibrary'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared library',
  robots: { index: false, follow: false }
}

export default async function ReadingDashboardPublicLibraryPage({
  params
}: {
  params: Promise<{ token: string }>
}) {
  const { token } = await params
  const client = await createServerClient(PublicReadingDashboardService)
  const library = await fetchOrNull(() => client.getSharedLibrary({ token }))

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[
          {
            label: library?.displayName ? `${library.displayName}'s reading` : 'Reading',
            href: `/dashboard/reading/${token}`
          },
          { label: 'Library' }
        ]}
      />

      <h1 className="mb-6 text-3xl font-bold">Library</h1>

      <SWRFallback fallback={library ? { [swrKeys.dashboardReading(token)]: library } : {}}>
        <ReadingDashboardPublicLibrary token={token} initialData={library ?? undefined} />
      </SWRFallback>
    </PageContainer>
  )
}
