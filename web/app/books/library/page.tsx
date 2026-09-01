import { Suspense } from 'react'
import Link from 'next/link'
import BooksSection from '@/components/books/BooksSection'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { PageContainer } from '@/components/ui/page-container'
import LibraryAdminButton from '@/components/books/LibraryAdminButton'

export default async function BacklogBooksLibraryPage() {
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Reading', href: '/dashboard/reading' }, { label: 'Library' }]}
      />

      <div className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-3xl font-bold">Library</h1>
        <div className="flex items-center gap-2">
          <LibraryAdminButton />
          <Button asChild variant="ghost" size="sm">
            <Link href="/feeds">Feed</Link>
          </Button>
          <Button asChild variant="ghost" size="sm" className="gap-2">
            <Link href="/books/settings">
              <SettingsIcon />
              Settings
            </Link>
          </Button>
        </div>
      </div>

      <SWRFallback fallback={library ? { [swrKeys.books]: library } : {}}>
        <Suspense fallback={<p className="text-muted">Loading…</p>}>
          <BooksSection />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
