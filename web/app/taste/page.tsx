import SWRFallback from '@/components/SWRFallback'
import TasteClient from '@/components/taste/TasteClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { TasteService } from '@/lib/gen/taste/v1/taste_pb'

export default async function TastePage() {
  const client = await createServerClient(TasteService)
  const [profile, stats, events] = await Promise.all([
    fetchOrNull(() => client.getProfile({})),
    fetchOrNull(() => client.getStats({})),
    fetchOrNull(() => client.listEvents({ domain: '', limit: DEFAULT_PAGE_SIZE }))
  ])

  return (
    <PageContainer>
      <SWRFallback
        fallback={{
          ...(profile && { [swrKeys.tasteProfile]: profile }),
          ...(stats && { [swrKeys.tasteStats]: stats })
        }}
        keyed={events ? [[swrKeys.tasteEvents(''), events]] : []}
      >
        <PageHeader title="Taste" />
        <TasteClient />
      </SWRFallback>
    </PageContainer>
  )
}
