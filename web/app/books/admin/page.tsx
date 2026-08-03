import BooksAdminClient from '@/components/books/BooksAdminClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { CatalogService } from '@/lib/gen/books/v1/catalog_pb'

export default async function BacklogBooksAdminPage() {
  const client = await createServerClient(CatalogService)
  const proposals = await fetchOrNull(() => client.listResyncProposals({}))

  return (
    <SWRFallback fallback={proposals ? { [swrKeys.resyncProposals]: proposals } : {}}>
      <BooksAdminClient />
    </SWRFallback>
  )
}
