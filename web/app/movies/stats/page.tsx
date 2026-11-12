import { useSharedList } from '@/hooks/useSharedList'
import { listHref } from '@/lib/listScope'
import AppSections from '@/components/AppSections'
import MoviesStatsClient from '@/components/movies/MoviesStatsClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function MoviesStatsPage() {
  const shared = useSharedList()

  return (
    <PageContainer>
      <PageHeader title="Movie stats" />
      <AppSections app="movies" hrefFor={(href) => listHref(href, shared)} />
      <MoviesStatsClient />
    </PageContainer>
  )
}
