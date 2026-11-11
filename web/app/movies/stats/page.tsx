import { useSharedList } from '@/hooks/useSharedList'
import { listHref } from '@/lib/listScope'
import MoviesStatsClient from '@/components/movies/MoviesStatsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function MoviesStatsPage() {
  const shared = useSharedList()

  return (
    <PageContainer>
      <PageHeader
        title="Movie stats"
        breadcrumb={[
          { label: 'Movies & series', href: listHref('/movies', shared) },
          { label: 'Stats' }
        ]}
      />
      <MoviesStatsClient />
    </PageContainer>
  )
}
