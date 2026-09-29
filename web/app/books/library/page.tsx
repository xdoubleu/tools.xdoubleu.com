import { Suspense } from 'react'
import BooksSection from '@/components/books/BooksSection'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'
import { PageContainer } from '@/components/ui/page-container'
import LibraryAdminButton from '@/components/books/LibraryAdminButton'

export default async function BacklogBooksLibraryPage() {
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))

  return (
    <PageContainer>
      <PageHeader
        breadcrumb={[{ label: 'Books', href: '/dashboard/reading' }, { label: 'Library' }]}
        title="Library"
        actions={
          <>
            <LibraryAdminButton />
            <PageHeaderSettingsLink href="/books/settings" />
          </>
        }
      />

      <SWRFallback fallback={library ? { [swrKeys.books]: library } : {}}>
        <Suspense fallback={<LoadingState />}>
          <BooksSection />
        </Suspense>
      </SWRFallback>
    </PageContainer>
  )
}
