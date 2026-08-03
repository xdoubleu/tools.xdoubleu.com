import AuthorBooksClient from '@/components/books/AuthorBooksClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'

export default async function AuthorBooksPage({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params
  const decoded = decodeURIComponent(name)
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))
  return (
    <SWRFallback fallback={library ? { [swrKeys.books]: library } : {}}>
      <AuthorBooksClient name={decoded} />
    </SWRFallback>
  )
}
