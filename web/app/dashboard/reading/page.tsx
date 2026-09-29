import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'
import ReadingDashboard from '@/components/dashboard/ReadingDashboard'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'

export default async function ReadingDashboardPage() {
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))

  return (
    <PageContainer className="lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden">
      <SWRFallback
        fallback={{
          ...(library ? { [swrKeys.books]: library } : {})
        }}
      >
        <PageHeader
          title="Books"
          className="mb-4 lg:mb-3"
          actions={<PageHeaderSettingsLink href="/books/settings" />}
        />

        <ReadingDashboard />
      </SWRFallback>
    </PageContainer>
  )
}
