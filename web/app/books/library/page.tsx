import BooksSection from '@/components/books/BooksSection'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default async function BacklogBooksLibraryPage() {
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Books', href: '/books' }, { label: 'Library' }]}
      />

      <h1 className="text-3xl font-bold mb-6">Library</h1>

      <SWRFallback fallback={library ? { [swrKeys.books]: library } : {}}>
        <BooksSection />
      </SWRFallback>
    </PageContainer>
  )
}
