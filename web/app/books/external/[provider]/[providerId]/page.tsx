import ExternalBookDetailClient from './ExternalBookDetailClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LibraryService } from '@/lib/gen/books/v1/library_pb'

export default async function ExternalBookDetailPage({
  params
}: {
  params: Promise<{ provider: string; providerId: string }>
}) {
  const { provider, providerId } = await params
  const client = await createServerClient(LibraryService)
  const result = await fetchOrNull(() => client.getExternalBook({ provider, providerId }))

  return (
    <SWRFallback
      fallback={{}}
      keyed={result ? [[swrKeys.externalBook(provider, providerId), result]] : []}
    >
      <ExternalBookDetailClient provider={provider} providerId={providerId} />
    </SWRFallback>
  )
}
