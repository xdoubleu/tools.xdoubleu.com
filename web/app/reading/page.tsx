import Link from 'next/link'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/reading/v1/library_pb'
import BooksDashboard from '@/components/reading/BooksDashboard'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { PageContainer } from '@/components/ui/page-container'

export default async function BacklogBooksPage() {
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))

  return (
    <PageContainer className="p-6 lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden lg:p-4">
      <div className="mb-4 flex items-center justify-between gap-4 lg:mb-3">
        <h1 className="text-3xl font-bold lg:text-2xl">Reading</h1>
        <Button asChild variant="ghost" size="sm" className="gap-2">
          <Link href="/reading/settings">
            <SettingsIcon />
            Settings
          </Link>
        </Button>
      </div>

      <SWRFallback fallback={library ? { [swrKeys.books]: library } : {}}>
        <BooksDashboard />
      </SWRFallback>
    </PageContainer>
  )
}
