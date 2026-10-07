import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { MoviesService } from '@/lib/gen/movies/v1/movies_pb'
import MoviesStatsClient from '@/components/movies/MoviesStatsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default async function MoviesStatsPage() {
  const client = await createServerClient(MoviesService)
  const stats = await fetchOrNull(() => client.getStats({}))

  return (
    <PageContainer>
      <SWRFallback fallback={stats ? { [swrKeys.moviesStats]: stats } : {}}>
        <PageHeader
          title="Stats"
          breadcrumb={[{ label: 'Movies & Series', href: '/movies' }, { label: 'Stats' }]}
        />
        <MoviesStatsClient />
      </SWRFallback>
    </PageContainer>
  )
}
