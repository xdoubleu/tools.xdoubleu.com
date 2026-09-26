import Link from 'next/link'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'
import ReadingDashboard from '@/components/dashboard/ReadingDashboard'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

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
          title="Reading"
          className="mb-4 lg:mb-3"
          actions={
            <Button asChild variant="ghost" size="sm" className="gap-2">
              <Link href="/books/settings">
                <SettingsIcon />
                Settings
              </Link>
            </Button>
          }
        />

        <ReadingDashboard />
      </SWRFallback>
    </PageContainer>
  )
}
