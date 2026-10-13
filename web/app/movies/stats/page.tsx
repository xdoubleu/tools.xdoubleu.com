import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { isSharedList, LIST_PARAM, listHref, sharedField } from '@/lib/listScope'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'
import MoviesStatsClient from '@/components/movies/MoviesStatsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default async function MoviesStatsPage({
  searchParams
}: {
  searchParams: Promise<{ [LIST_PARAM]?: string }>
}) {
  const shared = isSharedList((await searchParams)[LIST_PARAM])
  const client = await createServerClient(MoviesService)
  const stats = await fetchOrNull(() => client.getStats(sharedField(shared)))

  return (
    <PageContainer>
      <SWRFallback fallback={stats ? { [swrKeys.moviesStats(shared)]: stats } : {}}>
        <PageHeader
          title="Stats"
          breadcrumb={[
            { label: 'Movies & Series', href: listHref('/movies', shared) },
            { label: 'Stats' }
          ]}
        />
        <MoviesStatsClient />
      </SWRFallback>
    </PageContainer>
  )
}
