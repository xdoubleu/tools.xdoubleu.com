import { Suspense } from 'react'
import BookDetailClient from './BookDetailClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/reading/v1/library_pb'

export default async function BookDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))
  return (
    <SWRFallback fallback={library ? { [swrKeys.books]: library } : {}}>
      <Suspense fallback={<p className="p-6 text-muted">Loading…</p>}>
        <BookDetailClient id={id} />
      </Suspense>
    </SWRFallback>
  )
}
