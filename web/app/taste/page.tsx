import type { Arguments } from 'swr'
import SWRFallback from '@/components/SWRFallback'
import TasteClient from '@/components/taste/TasteClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { isSharedList, LIST_PARAM, sharedField } from '@/lib/listScope'
import { TasteService } from '@/lib/gen/taste/v1/taste_pb'

export default async function TastePage({
  searchParams
}: {
  searchParams: Promise<{ [LIST_PARAM]?: string }>
}) {
  const shared = isSharedList((await searchParams)[LIST_PARAM])
  const scope = sharedField(shared)
  const client = await createServerClient(TasteService)
  const [profile, recommendations, stats, events] = await Promise.all([
    fetchOrNull(() => client.getProfile(scope)),
    fetchOrNull(() => client.listRecommendations({ domain: '', track: 'more', ...scope })),
    fetchOrNull(() => client.getStats(scope)),
    fetchOrNull(() => client.listEvents({ domain: '', limit: DEFAULT_PAGE_SIZE, ...scope }))
  ])

  const keyed: [Arguments, unknown][] = []
  if (recommendations)
    keyed.push([swrKeys.tasteRecommendations('', 'more', shared), recommendations])
  if (events) keyed.push([swrKeys.tasteEvents('', shared), events])

  return (
    <PageContainer>
      <SWRFallback
        fallback={{
          ...(profile && { [swrKeys.tasteProfile(shared)]: profile }),
          ...(stats && { [swrKeys.tasteStats(shared)]: stats })
        }}
        keyed={keyed}
      >
        <PageHeader title="Taste" />
        <TasteClient />
      </SWRFallback>
    </PageContainer>
  )
}
