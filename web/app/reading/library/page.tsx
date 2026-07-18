import { Suspense } from 'react'
import Link from 'next/link'
import BooksSection from '@/components/reading/BooksSection'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/reading/v1/library_pb'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { PageContainer } from '@/components/ui/page-container'

export default async function BacklogBooksLibraryPage() {
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Reading', href: '/reading' }, { label: 'Library' }]}
      />

      <div className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-3xl font-bold">Library</h1>
        <Button asChild variant="ghost" size="sm" className="gap-2">
          <Link href="/reading/settings">
            <SettingsIcon />
            Settings
          </Link>
        </Button>
      </div>

      <SWRFallback fallback={library ? { [swrKeys.books]: library } : {}}>
        <Suspense fallback={<p className="text-muted">Loading…</p>}>
          <BooksSection />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
