import { Suspense } from 'react'
import ReadBookClient from './ReadBookClient'
import { LoadingState } from '@/components/ui/states'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'

export default async function ReadBookPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const client = await createServerClient(LibraryService)
  const library = await fetchOrNull(() => client.getLibrary({}))
  return (
    <SWRFallback fallback={library ? { [swrKeys.books]: library } : {}}>
      <Suspense fallback={<LoadingState className="p-6" />}>
        <ReadBookClient id={id} />
      </Suspense>
    </SWRFallback>
  )
}
